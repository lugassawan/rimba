package example

// Package-level struct is fine.
type Good struct{ x int }

func bad() {
	type Local struct{ x int } // want `named struct type "Local" should be declared at package level, not inside a function`
	_ = Local{}
}

func alsoOk() {
	// Non-struct local types are allowed.
	type myInt int
	_ = myInt(0)
}

var fn = func() {
	type InLiteral struct{ y int } // want `named struct type "InLiteral" should be declared at package level, not inside a function`
	_ = InLiteral{}
}
