package example

const Early = "ok" // OK - before any function

var x = 1 // vars are fine anywhere

func foo() {}

const Late = "bad" // want `const declaration should appear before all function declarations`

func bar() {}

const AlsoLate = "bad" // want `const declaration should appear before all function declarations`
