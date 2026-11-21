// Package api re-exports the types of real under other names, as a client
// library does with the types of its internal package.
package api

import "example.com/cases/cases/typealias/real"

type AliasA = real.A

type AliasB = real.B

type AliasC = real.C

type AliasD = real.D

type AliasE = real.E

type AliasF = real.F

type AliasG = real.G

type AliasH = real.H

type PtrH = *real.H

type ListH = []PtrH

type MapH = map[string]PtrH

type PCT = *real.CT
