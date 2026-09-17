package main

// progressName is the name the progress is saved under, with golib.SaveData.
const progressName = "progress"

// progress is what the game remembers between runs: the levels finished,
// with the fewest moves each took, and whether the music is off.
type progress struct {
	Best     map[string]int `json:"best"`     // fewest moves for each finished level, by map file
	MusicOff bool           `json:"musicOff"` // off, rather than on, so that nothing saved means music
}

// finished reports whether level l has been finished at least once.
func (p *progress) finished(l *layout) bool {
	_, ok := p.Best[l.name]
	return ok
}

// record notes that level l was finished in moves. It reports whether that
// is the level's best so far, which the first finish always is.
func (p *progress) record(l *layout, moves int) bool {
	if best, ok := p.Best[l.name]; ok && best <= moves {
		return false
	}
	if p.Best == nil {
		p.Best = map[string]int{}
	}
	p.Best[l.name] = moves
	return true
}
