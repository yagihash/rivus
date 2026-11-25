package taint

import "github.com/yagihash/rivus/internal/model"

// UnitOf is what a function ID says about the unit of code it belongs to (model.UnitOf).
func UnitOf(funcID string) string { return model.UnitOf(funcID) }
