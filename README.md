# hc

hc is a CLI tool for defining and running HTTP API tests using .http files.

## Installation

```sh
go install github.com/skranpn/hc/cmd/hc@latest
```

## Quick Start

Create a file `api.http`:

```http
# @name GET
GET https://example.com

# @assert GET.response.status == 200
```

Run it:

```sh
hc run api.http
```

## .http File Syntax

### Request

Each request consists of a request line, optional headers, and an optional body separated by a blank line.

```http
POST https://example.com/todos
Content-Type: application/json

{
    "title": "Buy groceries",
    "completed": false
}
```

Supported HTTP methods: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS.

The HTTP version (HTTP/1.1) may optionally be appended to the request line.

### Multiple Requests

Use `###` to separate multiple requests in a single file. Requests are executed top to bottom.

```http
# @name CreateTodo

POST https://example.com/todos
Content-Type: application/json

{
    "title": "Buy groceries",
    "completed": false
}

# @id = {{CreateTodo.response.body.id}}

###

GET https://example.com/todos/{{id}}
```

### Metadata

Metadata lines are placed after the request (before the next `###`). They can be written with `# @`, `// @`, or `@` prefixes.

#### `@name` Request name

Give a request name to reference its response in later requests.

```http
# @name CreateTodo
POST https://example.com/todos
Content-Type: application/json

{
    "title": "Buy groceries",
    "completed": false
}
```

#### `@assert` Assertion

Assert a condition against the response. Failed assertions are reported but do not stop execution by default.

```
# @assert <path> <operator> <value>
```

```http
# @name GetTodos
GET https://example.com/todos

# @assert GetTodos.response.status == 200
# @assert GetTodos.response.body is array
```

Operators:

| Operator   | Description                                            |
| ---------- | ------------------------------------------------------ |
| `==`       | Equal                                                  |
| `!=`       | Not equal                                              |
| `<`        | Less than (numeric)                                    |
| `>`        | Greater than (numeric)                                 |
| `<=`       | Less than or equal (numeric)                           |
| `>=`       | Greater than or equal (numeric)                        |
| `contains` | String contains                                        |
| `is`       | Type check (array, object, string, number, bool, null) |

#### `@<name> = <value>` Variable assignment

Extract a value from the response and store it as a variable for use in subsequent requests.

```http
# @name CreateTodo
POST https://example.com/todos
Content-Type: application/json

{"title": "Task 1"}

@todoId = {{CreateTodo.response.body.id}}
```

#### `@until` Retry until condition

Retry the request until the condition is met, up to a maximum number of attempts.

```
# @until <path> <operator> <value> max <n> [interval <n>]
```

```http
# @name WaitForJob
GET https://example.com/todos/{{todoId}}

# @until WaitForJob.response.body.status == done max 10 interval 5
# @assert WaitForJob.response.status == 200
```

`interval` accepts a plain integer (seconds) or a duration string (e.g. 5s, 500ms)

#### `@skip` Skip request conditionally

Skip the request when the condition is true.

```
# @skip if <path> <operator> <value>
```

```http
GET https://example.com/todos

# @skip if {{ENV}} == dev
```

### Variables

#### Env file

Load variables from a .env-style file with `-e`:

```sh
hc run -e .env api.http
```

.env file format:

```
BASE_URL=https://example.com
ENV=dev
```

#### Previous response reference

Reference a value from a previous response using its request name and JSONPath.

#### System variables

| Variable                     | Description                                      |
| ---------------------------- | ------------------------------------------------ |
| `{{$guid}}`                  | Random UUID v4                                   |
| `{{$randomInt <min> <max>}}` | Random integer between min and max               |
| `{{$timestamp}}`             | Current Unix timestamp (seconds)                 |
| `{{$timestamp <n> <unit>}}`  | Unix timestamp with offset (s, m, h, d, w, M, y) |

### JSONPath Reference

Paths used in `@assert`, `@until`, `@skip`, and variable assignment follow this format:

```
<RequestName>.response.status
<RequestName>.response.body.<field>
<RequestName>.response.headers.<header-name>
```

## CLI Options

```sh
hc run [flags] <http_file>
```

| Flag              | Short | Default | Description                                                                    |
| ----------------- | ----- | ------- | ------------------------------------------------------------------------------ |
| --env             | -e    |         | Path to env file                                                               |
| --proxy           | -p    |         | Proxy URL                                                                      |
| --out             | -o    | out     | Output directory for results                                                   |
| --interval        | -i    | 1000    | Interval between requests (milliseconds)                                       |
| --stop-on-failure |       | false   | Stop execution on assertion failure                                            |
| --stop-on-error   |       | false   | Stop execution on any error                                                    |
| --parallel        |       | false   | (Experimental) Enable parallel execution (respects inter-request dependencies) |
| --jobs            | -j    | 4       | Max concurrent requests when --parallel is enabled                             |
| --request-timeout |       | 30      | Timeout per request in seconds (0 = no timeout)                                |
| --total-timeout   |       | 0       | Total execution timeout in seconds (0 = no timeout)                            |

### Interactive controls

While running, press:

- Space: pause / resume execution
- Ctrl+C: cancel execution

### Output Files

Each run writes results to `<out>/<unix-timestamp>/`:

| File         | Description                                                         |
| ------------ | ------------------------------------------------------------------- |
| last.txt     | Raw request and response of the most recently executed request      |
| result.txt   | Accumulated raw requests and responses for all executed requests    |
| summary.md   | Markdown table summarizing all request results and assertion status |
| variable.txt | Snapshot of all variables at the end of execution                   |
