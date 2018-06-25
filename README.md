# pg2any

pg2any is a small tool which inspects PostgreSQL information and outputs via various kind of format.

CAUTION: This tool is not aiming to be widely used. This has only tiny features.

Current support outputs:

- hibernate (JPA)
- sphinx (reStrcuturedText)
- protobuf (protocol buffer)


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
      "type": "sphinx",
      "output": "docs",
      "templates": "templates/sphinx",
      "ignore_tables": [
        "flyway_schema_history"
      ],
      "use_string_to_numeric": true
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
- ignore_tables: list of ignore table.

tips: To add toctree, `:glob:` is useful.

## protobuf config

Protobuf generator outputs tables as `message`.

- type: must be "protobuf".
- output: output directory.
- templates: template directory.
- package_name: package name.
- ignore_tables: list of ignore table.
- use_string_to_numeric: if true, use `string` instead of `int64` on numeric type

# Thanks

- https://github.com/achiku/dgw
- https://github.com/xo/xo
- https://github.com/volatiletech/sqlboiler/

# License

MIT
