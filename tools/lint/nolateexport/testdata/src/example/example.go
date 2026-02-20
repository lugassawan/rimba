package example

type T struct{}

func init() {} // OK - init is exempt

func Exported() {} // OK - exported before unexported

func unexported() {} // OK - unexported after exported

func (t T) ExportedMethod() {} // OK - methods are ignored

func AlsoExported() {} // want `exported function AlsoExported should appear before unexported functions`

func init() {} // OK - second init is also exempt

func alsoUnexported() {} // OK
