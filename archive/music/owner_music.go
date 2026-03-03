//go:build ignore

package archive_music

import "strings"

// Archived owner-side command wrapper for `.music`.

func musicMenu(prefix string) string {
	if prefix == "" {
		prefix = "."
	}
	return "" +
		"🎵 *MUSIC CONTROL*\n\n" +
		"Now Playing\n" +
		"- " + prefix + "music now\n" +
		"- " + prefix + "music status\n\n" +
		"Library/Search\n" +
		"- " + prefix + "music list 20\n" +
		"- " + prefix + "music list playlist Favorites 30\n" +
		"- " + prefix + "music search coldplay\n" +
		"- " + prefix + "music searchall breath 40\n" +
		"- " + prefix + "music osearch weeknd\n" +
		"- " + prefix + "music oplay 1\n\n" +
		"Play/Queue\n" +
		"- " + prefix + "music play 3\n" +
		"- " + prefix + "music playq 1\n" +
		"- " + prefix + "music queue add 2\n" +
		"- " + prefix + "music queue show\n" +
		"- " + prefix + "music queue clear\n\n" +
		"Playback\n" +
		"- " + prefix + "music pause | toggle | next | prev\n" +
		"- " + prefix + "music vol 50\n" +
		"- " + prefix + "music shuffle on\n" +
		"- " + prefix + "music repeat all"
}

func normalizeMusicArg(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

