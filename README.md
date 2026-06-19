# alpakr

YAML-configured data transformation tool. Reads JSON or YAML from a file or URL, applies a pipeline of named handlers to restructure and transform the data, and outputs JSON or YAML to stdout or a file.

Useful for: migrating between schemas, importing externally sourced data, normalising nested/relational data.

## Install

```sh
go install alpakr@latest
# or
make build   # outputs to ./bin/alpakr
```

## Usage

```sh
# alpakr looks for alpakr.yaml or alpakr.yml in the current directory by default
alpakr run                             # uses alpakr.yaml/yml in cwd, defaults to 'root' handler
alpakr run --handler staff             # run a specific handler
alpakr run -c path/to/config.yaml      # explicit config path
alpakr run --format yaml               # override output format
alpakr run -o out.json                 # write to file instead of stdout

alpakr validate                        # check config + compile all expressions
alpakr list-handlers                   # print handler names defined in config

alpakr run | jq .                      # pipe to other tools
```

## Config

### Single source

```yaml
version: "1"

source:
  path: ./data/records.json   # local file — mutually exclusive with url:
  # url: https://example.com/data.json
  # format: json              # json | yaml — auto-detected from extension

output:
  format: json                # json | yaml (default: json)
  indent: 2                   # JSON indentation (default: 2)
  # file: ./out/result.json   # write to file instead of stdout

handlers:
  root:                       # 'root' is used by default if no --handler given
    input: ".data"            # jq selector applied to raw source before processing
    each: true                # iterate array input, run handler per element
    filter: ".active == true" # jq predicate — records that evaluate falsy are dropped
    fields:
      id:    ".id"
      name:  ".name | ascii_upcase"
      score: ".raw_score * 10 | round2"
      tags:  "[.tags[] | ascii_downcase]"
      location:
        handler: place        # delegate this field to another handler
        input: ".loc"         # jq selector to extract input for sub-handler

  place:
    fields:
      city:    ".city"
      country: ".country_code | ascii_upcase"
```

### Multiple sources

Use a `sources` map and assign each handler a `source:` key. Handlers with different sources can coexist in one config file — each `alpakr run --handler <name>` loads only that handler's source.

```yaml
version: "1"

sources:
  staff:
    path: ./data/staff.json
  projects:
    path: ./data/projects.yaml   # format auto-detected from extension
  inventory:
    url: https://example.com/inventory.json

output:
  format: json
  indent: 2

handlers:
  staff:
    source: staff
    each: true
    fields:
      id:   ".emp_id"
      name: '.first + " " + .last'

  projects:
    source: projects
    input: ".projects"
    each: true
    filter: '.status == "active"'
    fields:
      id:   ".id"
      name: ".name"

  inventory:
    source: inventory
    each: true
    fields:
      sku:   ".sku"
      stock: ".qty"
```

When no `root` handler is defined, `--handler` is required. The error message lists available handler names.

### Field values

Each field value is either:

- A **jq expression** string — evaluated against the current record
- A **sub-handler reference** — delegates to another handler, enabling nested/relational data without duplication

```yaml
fields:
  # jq expression
  title: ".name | ascii_downcase"

  # sub-handler reference
  location:
    handler: place   # name of handler to run
    input: ".loc"    # jq selector to extract input (defaults to .)
```

### Nested data

Handlers compose recursively. A field in one handler can delegate to another handler, which can itself delegate further. This avoids duplicating field mappings for shared structures.

```yaml
handlers:
  root:
    each: true
    fields:
      title: ".name"
      county:
        handler: county
        input: ".location"

  county:
    fields:
      name: ".county"
      country:
        handler: country
        input: "."

  country:
    fields:
      name: ".country"
      code: ".country_code | ascii_upcase"
```

### Nested collection input

If the source wraps the collection in an object, use `input` to extract it first:

```yaml
handlers:
  root:
    input: ".data.records"   # extract the array before each iterates
    each: true
    fields:
      id: ".id"
```

## Built-in transform functions

All standard [jq](https://jqlang.github.io/jq/manual/) functions are available, plus these extras:

| Function | Description | Example |
|---|---|---|
| `round2` | Round float to 2 decimal places | `.miles * 1.60934 \| round2` |
| `slugify` | Lowercase, spaces→dashes, strip non-alphanumeric | `.name \| slugify` |
| `to_int` | Convert string or float to integer | `.score \| to_int` |
| `to_float` | Convert string or integer to float | `.count \| to_float` |

### Common jq patterns

```yaml
# String operations
title:     ".name | ascii_downcase"
upper:     ".code | ascii_upcase"
trimmed:   '.label | gsub("^\\s+|\\s+$"; "")'
replaced:  '.text | gsub("foo"; "bar")'

# Math
km:        ".miles * 1.60934 | round2"
remaining: ".budget - .spent"
pct:       "(.spent / .budget * 100) | round2"

# Date formatting
date:      '.iso_date | strptime("%Y-%m-%d") | strftime("%d/%m/%Y")'
from_unix: ".created_ts | todate"

# Arrays
tags:      "[.tags[] | ascii_upcase]"
count:     ".items | length"
total:     "([.lines[] | .qty * .price] | add) | round2"

# Conditionals
type:      'if .kind == "A" then "Alpha" else "Other" end'
street:    '.line1 + (if .line2 != "" then ", " + .line2 else "" end)'

# Null coalescing
label:     '.name // "unknown"'

# Computed / concatenated
full_name: '.first + " " + .last'
```

## Example

Source (`data/outings.json`):

```json
[
  {
    "id": 1,
    "name": "Peak District Walk",
    "date": "2024-03-15",
    "distance_miles": 8.5,
    "tags": ["hiking", "hills"],
    "location": { "county": "Derbyshire", "country": "England", "country_code": "gb" }
  }
]
```

Config (`alpakr.yaml`):

```yaml
version: "1"

source:
  path: ./data/outings.json

output:
  format: json
  indent: 2

handlers:
  root:
    each: true
    filter: ".distance_miles > 0"
    fields:
      id:          ".id"
      title:       ".name | ascii_downcase"
      date:        '.date | strptime("%Y-%m-%d") | strftime("%d/%m/%Y")'
      distance_km: ".distance_miles * 1.60934 | round2"
      tags:        "[.tags[] | ascii_upcase]"
      location:
        handler: county
        input: ".location"

  county:
    fields:
      name:    ".county"
      slug:    ".county | slugify"
      country:
        handler: country
        input: "."

  country:
    fields:
      name: ".country"
      code: ".country_code | ascii_upcase"
```

Output:

```json
[
  {
    "date": "15/03/2024",
    "distance_km": 13.68,
    "id": 1,
    "location": {
      "country": { "code": "GB", "name": "England" },
      "name": "Derbyshire",
      "slug": "derbyshire"
    },
    "tags": ["HIKING", "HILLS"],
    "title": "peak district walk"
  }
]
```

The [`examples/`](./examples) directory contains a fully annotated reference config at [`examples/alpakr.yaml`](./examples/alpakr.yaml) — every option documented with explanation, rules, defaults, and usage examples. Further working examples covering different data shapes and config patterns are in the subdirectories.

## Development

```sh
make build    # build ./bin/alpakr
make test     # run tests
make test-v   # run tests verbose
make lint     # go vet
make tidy     # go mod tidy
make clean    # remove build artifacts
```
