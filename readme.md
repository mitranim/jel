## Overview

"JSON Expession Language". Expresses a whitelisted subset of SQL with simple JSON structures. Transcodes JSON queries to SQL.

See the full documentation at https://godoc.org/github.com/mitranim/jel.

See the sibling library https://github.com/mitranim/sqlb for SQL query building.

**This repo is deprecated: the code has been merged into https://github.com/mitranim/sqlb.**

## Changelog

### 0.2.0

Update to match the recent breaking changes in the `sqlb` package.

### 0.1.3

Breaking: removed `Ord` after moving it to `sqlb`, which is a dependency of this package.

### 0.1.2

Minor breaking change: `Ord` now uses the `nulls last` qualifier. We might want to make this configurable in the future.

### 0.1.1

Added `Ords` for SQL `order by`.

The new type `Ords` represents an SQL `order by` clause in a structured fashion, and allows to safely decode it from client input. Just like `Expr`, decoding `Ords` is performed by consulting a user-specified struct type. JSON field names are converted to DB column names, unknown fields cause a parse error. When encoding for SQL, identifiers are quoted for safety.

Minor breaking change: renamed `ExprFrom` → `ExprFor`.

### 0.1.0

First tagged release.

## License

https://unlicense.org

## Misc

I'm receptive to suggestions. If this library _almost_ satisfies you but needs changes, open an issue or chat me up. Contacts: https://mitranim.com/#contacts
