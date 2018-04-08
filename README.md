# pg2any

pg2any is a small tool which inspect PostgreSQL information and output various kind of format.


Current support output:

- hibernate (JPA)
- protobuf (protocol buffer)
- sphinx (reStrcuturedText)  (but not implemented yet)


# config

You can specify `-c` option or if not specified, pg2any search same directory.

```
{
  "src": "user=postgres dbname=foo sslmode=disable password=VerySecret",
  "generators": [
    {
      "type": "hibernate",
      "output": "src/main/java/com/foo/bar/entity",
      "templates": "templates/hibernate",
      "package_name": "com.foo.bar.entity",
      "ignore_tables": [
        "flyway_schema_history"
      ]
    },
    {
      "type": "protobuf",
      "output": "src/proto",
      "templates": "templates/protobuf",
      "ignore_tables": [
        "flyway_schema_history"
      ]
    }
  ]
}
```

## hibernate config

- type: must be "hibernate".
- output: output directory.
- templates: template directory.
- package_name: package name.
- ignore_tables: list of ignore table.
- read_only_columns: list of getter only columns.

## sphinx config

- type: must be "sphinx".
- output: output directory.
- templates: template directory.

## protobuf config

- type: must be "protobuf".
- output: output directory.
- templates: template directory.
- package_name: package name.
- ignore_tables: list of ignore table.

# Thanks

- https://github.com/achiku/dgw
- https://github.com/xo/xo
- https://github.com/volatiletech/sqlboiler/

# License

MIT
