### Fields with nested structs vs pointers
**TL;DR**: there's no single best solution to whether you should define nested structs by their pointer or not. Most people will agree to use pointer when said struct is large, but otherwise it's mostly down to preference. Sometimes people think that using pointers should be faster as no data is being copied but it's not always true - especially in languages with GC like Golang. Depending how Go's compiler decides, your data may be swapped between stack and heap memories based on how it thinks will be more efficient.

In our case - I've decided to by default pass data by value (copy them) as it remains very cheap and in some edge cases can help. It feels like force using pointers everything is too risky. So, only use pointer if it's **optional field** or when copied struct is large (size >= 600). We're talking here about single instances, please don't use pointer to arrays/slices or maps as they are already giving you just a pointer.


### JSON Parsing of Optional Fields
To ensure consistent behavior with other languages and align with Discord API expectations, Tempest follows a specific convention for optional fields in Go:
- `omitempty` for optional fields like strings, numbers, etc.
- `omitzero` for **slices or maps** where an **explicit empty value** (e.g., `[]`) must be included in the payload to signal the removal or absence of a resource.

> ⚠️ Please at least for now avoid using `omitempty` on bool type struct fields. Go's default, zero value logic may overwrite expected outcome (in some cases). I'll try finding better solution later on but that works for now.


### API Documentation Links
Always try to include direct URL links to the relevant pieces of the Top.gg documentation (e.g., `https://docs.top.gg/api/v1/...`) in the doc comments directly above any corresponding structs, payload types, or methods. This helps users quickly cross-reference SDK behavior with the official API docs.
