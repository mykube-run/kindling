package main

// This example demonstrates the usage of kconfig.
// Usage:
// - 1. Start a config source, e.g. Consul, Etcd, Nacos (or create a config file).
// - 2. Populate initial config with value like '{"int": 36}'
// - 3. Start the example, e.g.:
// 		- `go run ./cmd/kconfig-example --conf-src-addr='localhost:8848' --conf-src-format=json --conf-src-namespace=kconfig --conf-src-group=test --conf-src-key=kconfig-test --conf-src-type=nacos`
// - 4. Observe log output
// - 5. Update config content and observe log output
