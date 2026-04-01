# Datacow

Muh'ing for data.

Datacow is a tool to visualize, navigate and search data in relational databases.

- Something like metabase, but only more raw, with support for different 
data sources and combining them.
- Provides a "core" which contains the abstractions over the raw RDBMS objects and expose them to different parts of datacow interfaces:
  - an HTTP-based API
  - a TUI CLI, with slash commands, history, completions, advanced line editing etc.
  - a webapp


# Model Glossary

* `datasource`: A connection to a database (RDBMS, Mongo, an API, ...)
* `dataset`: A view of data in a datasource, could simply be a table (select all rows and columns) or a custom written query spanning tables
 
# Ideas To Try Out

* `directory based CLI navigation` - does it make sense to "cd"-into tables or datasets and "ls" the contents?
