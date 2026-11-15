# fontresource

Immutable font bytes and versioned provenance, without parsing or discovery.

```go
resource, err := fontresource.Read(reader, fontresource.Origin{
    Provider: "file", Location: path,
})
if err != nil {
    return err
}
retained := resource.Open()
defer retained.Close()
```

`Read` copies input into a snapshot, rejects content larger than 64 MiB and does
not close the input reader. `Open` returns an independent reader without exposing
mutable bytes. Copies of a resource may safely share its private immutable data.

`SHA256` hashes content. `ID` additionally includes quoted provenance fields and
legacy request codes using the fixed version-1 encoding. Slant/weight codes are
not numeric CSS weights; adapters perform conversion. Do not change the encoding
without a versioned migration. A zero resource is empty; parsing validity belongs
to the backend adapter.

`resource_test.go` covers snapshots and identities; run `go test ./fontresource`.
See the [integration guide](../README.md) for pinned snapshots and replay policy.
