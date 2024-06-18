Fork for using sobek

----

Nodejs compatibility library for Sobek
====

This is a collection of Goja modules that provide nodejs compatibility.

Example:

```go
package main

import (
    "github.com/grafana/sobek"
    "github.com/practice-golang/goja_nodejs/require"
)

func main() {
    registry := new(require.Registry) // this can be shared by multiple runtimes

    runtime := goja.New()
    req := registry.Enable(runtime)

    runtime.RunString(`
    var m = require("./m.js");
    m.test();
    `)

    m, err := req.Require("./m.js")
    _, _ = m, err
}
```

More modules will be added. Contributions welcome too.
