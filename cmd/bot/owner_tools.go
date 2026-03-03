package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	execTimeout      = 15 * time.Second
	maxExecReplySize = 3500
)

func (b *Bot) RunExec(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command kosong")
	}
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	result := trimExecOutput(string(out))
	if ctx.Err() == context.DeadlineExceeded {
		return result, fmt.Errorf("timeout setelah %s", execTimeout)
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func (b *Bot) EvalExpression(expr string) (string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", fmt.Errorf("expression kosong")
	}
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return "", err
	}
	v, err := evalNode(node)
	if err != nil {
		return "", err
	}
	return formatEvalValue(v), nil
}

func trimExecOutput(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	runes := []rune(out)
	if len(runes) <= maxExecReplySize {
		return out
	}
	return string(runes[:maxExecReplySize]) + "\n...(output dipotong)"
}

func evalNode(node ast.Expr) (any, error) {
	switch n := node.(type) {
	case *ast.ParenExpr:
		return evalNode(n.X)
	case *ast.BasicLit:
		switch n.Kind {
		case token.INT:
			v, err := strconv.ParseInt(n.Value, 10, 64)
			if err != nil {
				return nil, err
			}
			return float64(v), nil
		case token.FLOAT:
			v, err := strconv.ParseFloat(n.Value, 64)
			if err != nil {
				return nil, err
			}
			return v, nil
		case token.STRING:
			v, err := strconv.Unquote(n.Value)
			if err != nil {
				return nil, err
			}
			return v, nil
		default:
			return nil, fmt.Errorf("literal tidak didukung")
		}
	case *ast.Ident:
		switch n.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, fmt.Errorf("identifier tidak didukung: %s", n.Name)
		}
	case *ast.UnaryExpr:
		x, err := evalNode(n.X)
		if err != nil {
			return nil, err
		}
		switch n.Op {
		case token.ADD:
			f, ok := toFloat(x)
			if !ok {
				return nil, fmt.Errorf("operator + unary butuh angka")
			}
			return f, nil
		case token.SUB:
			f, ok := toFloat(x)
			if !ok {
				return nil, fmt.Errorf("operator - unary butuh angka")
			}
			return -f, nil
		case token.NOT:
			b, ok := x.(bool)
			if !ok {
				return nil, fmt.Errorf("operator ! butuh bool")
			}
			return !b, nil
		default:
			return nil, fmt.Errorf("operator unary tidak didukung: %s", n.Op.String())
		}
	case *ast.BinaryExpr:
		left, err := evalNode(n.X)
		if err != nil {
			return nil, err
		}
		right, err := evalNode(n.Y)
		if err != nil {
			return nil, err
		}
		return applyBinary(n.Op, left, right)
	default:
		return nil, fmt.Errorf("expression tidak didukung")
	}
}

func applyBinary(op token.Token, left, right any) (any, error) {
	switch op {
	case token.ADD:
		if ls, ok := left.(string); ok {
			rs, ok2 := right.(string)
			if !ok2 {
				return nil, fmt.Errorf("string hanya bisa ditambah string")
			}
			return ls + rs, nil
		}
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if !lok || !rok {
			return nil, fmt.Errorf("operator + butuh angka atau string")
		}
		return lf + rf, nil
	case token.SUB, token.MUL, token.QUO:
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if !lok || !rok {
			return nil, fmt.Errorf("operator %s butuh angka", op.String())
		}
		switch op {
		case token.SUB:
			return lf - rf, nil
		case token.MUL:
			return lf * rf, nil
		case token.QUO:
			if rf == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return lf / rf, nil
		}
	case token.REM:
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if !lok || !rok {
			return nil, fmt.Errorf("operator %% butuh angka")
		}
		if rf == 0 {
			return nil, fmt.Errorf("mod by zero")
		}
		if !isWholeNumber(lf) || !isWholeNumber(rf) {
			return nil, fmt.Errorf("operator %% butuh angka bulat")
		}
		return float64(int64(lf) % int64(rf)), nil
	case token.LAND, token.LOR:
		lb, ok := left.(bool)
		if !ok {
			return nil, fmt.Errorf("operator %s butuh bool", op.String())
		}
		rb, ok := right.(bool)
		if !ok {
			return nil, fmt.Errorf("operator %s butuh bool", op.String())
		}
		if op == token.LAND {
			return lb && rb, nil
		}
		return lb || rb, nil
	case token.EQL, token.NEQ:
		eq, err := compareEqual(left, right)
		if err != nil {
			return nil, err
		}
		if op == token.EQL {
			return eq, nil
		}
		return !eq, nil
	case token.GTR, token.GEQ, token.LSS, token.LEQ:
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if lok && rok {
			switch op {
			case token.GTR:
				return lf > rf, nil
			case token.GEQ:
				return lf >= rf, nil
			case token.LSS:
				return lf < rf, nil
			case token.LEQ:
				return lf <= rf, nil
			}
		}
		ls, lok := left.(string)
		rs, rok := right.(string)
		if lok && rok {
			switch op {
			case token.GTR:
				return ls > rs, nil
			case token.GEQ:
				return ls >= rs, nil
			case token.LSS:
				return ls < rs, nil
			case token.LEQ:
				return ls <= rs, nil
			}
		}
		return nil, fmt.Errorf("operator %s butuh angka atau string", op.String())
	}
	return nil, fmt.Errorf("operator tidak didukung: %s", op.String())
}

func compareEqual(left, right any) (bool, error) {
	if lf, lok := toFloat(left); lok {
		if rf, rok := toFloat(right); rok {
			return lf == rf, nil
		}
	}
	switch l := left.(type) {
	case string:
		r, ok := right.(string)
		if !ok {
			return false, nil
		}
		return l == r, nil
	case bool:
		r, ok := right.(bool)
		if !ok {
			return false, nil
		}
		return l == r, nil
	default:
		return false, fmt.Errorf("tipe tidak didukung untuk perbandingan")
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func isWholeNumber(v float64) bool {
	return math.Abs(v-math.Round(v)) < 1e-9
}

func formatEvalValue(v any) string {
	switch x := v.(type) {
	case float64:
		if isWholeNumber(x) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}
