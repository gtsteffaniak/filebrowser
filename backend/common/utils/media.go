package utils

type SubtitleTrack struct {
	Name     string `json:"name"`            // filename for external, or descriptive name for embedded
	Language string `json:"language"`        // language code
	Title    string `json:"title,omitempty"` // title/description
	Index    *int   `json:"index,omitempty"` // stream index for embedded subtitles (nil for external)
	Codec    string `json:"codec,omitempty"` // codec name for embedded subtitles
	IsFile   bool   `json:"isFile"`          // true for external files, false for embedded
}
