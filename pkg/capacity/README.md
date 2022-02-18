To benchmark the CSIStorageCapacity update code, use:

```
KUBECONFIG=<some config file> go test -bench=. -run=xxx .
```

Running repeatedly with `-count=5` and filtering the output with
[benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) is recommended
to determine how stable the results are.
