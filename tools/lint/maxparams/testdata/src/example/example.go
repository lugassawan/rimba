package example

// ok7 has exactly 7 parameters — at the limit, should pass.
func ok7(a, b, c, d, e, f, g int) {}

// bad8 has 8 parameters — over the limit.
func bad8(a, b, c, d, e, f, g, h int) {} // want `bad8 has 8 parameters, max is 7`

// bad9mixed has 9 parameters with mixed types.
func bad9mixed(a, b int, c string, d, e, f bool, g, h, i float64) {} // want `bad9mixed has 9 parameters, max is 7`

// okGrouped has 6 parameters grouped into 2 fields.
func okGrouped(a, b, c int, d, e, f string) {}

// Function literals are also checked.
var badLit = func(a, b, c, d, e, f, g, h int) {} // want `function literal has 8 parameters, max is 7`

var okLit = func(a, b, c int) {}
