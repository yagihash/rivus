package main

import (
	"encoding/json"
	"os"
)

// Digest is what the table needs of one `rivus analyze -pass-stats` result.
type Digest struct {
	Name     string `json:"name"`
	Lang     string `json:"lang"`
	Findings int    `json:"findings"`
	Funcs    int    `json:"funcs"`
	// Uncertainties counts by kind.
	Uncertainties map[string]int `json:"uncertainties"`
	// Unmodeled maps a symbol to the number of unmodeled_library_call (and reflection) sites.
	Unmodeled map[string]int `json:"unmodeled"`
	// NoSymbol is the unmodeled library calls without a symbol.
	NoSymbol int `json:"no_symbol"`
	// Top is stats.unmodeled_top: the 20 functions without a model that tainted values reached the most.
	Top []struct {
		Symbol  string `json:"symbol"`
		Reached int    `json:"reached"`
		OnPath  int    `json:"on_path"`
	} `json:"top"`
	Converged *bool  `json:"converged,omitempty"`
	Note      string `json:"note,omitempty"`
}

func digestFile(path, name, lang string) (*Digest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r struct {
		Findings      []json.RawMessage `json:"findings"`
		Uncertainties []struct {
			Kind   string `json:"kind"`
			Symbol string `json:"symbol"`
		} `json:"uncertainties"`
		Stats struct {
			Functions    int  `json:"functions"`
			Converged    bool `json:"converged"`
			UnmodeledTop []struct {
				Symbol  string `json:"symbol"`
				Reached int    `json:"reached"`
				OnPath  int    `json:"on_path"`
			} `json:"unmodeled_top"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	d := &Digest{Name: name, Lang: lang, Findings: len(r.Findings), Funcs: r.Stats.Functions,
		Uncertainties: map[string]int{}, Unmodeled: map[string]int{}}
	conv := r.Stats.Converged
	d.Converged = &conv
	for _, u := range r.Uncertainties {
		d.Uncertainties[u.Kind]++
		if u.Kind == "unmodeled_library_call" || u.Kind == "reflection" {
			if u.Symbol == "" {
				d.NoSymbol++
			} else {
				d.Unmodeled[u.Symbol]++
			}
		}
	}
	d.Top = r.Stats.UnmodeledTop
	return d, nil
}

func loadDigest(path string) (*Digest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d Digest
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return &d, nil
}
