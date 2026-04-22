# Markdown Extensions

gomddoc extends standard GitHub Flavored Markdown (GFM) with additional features for documentation sites.

## Admonitions (Callout Blocks)

Admonitions are styled callout blocks that highlight important information. They use the same syntax as [GitHub's alerts](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#alerts).

### Syntax

Write a blockquote that starts with a type marker on the first line:

```markdown
> [!NOTE]
> This is a note with useful information.

> [!TIP]
> This is a helpful tip for the reader.

> [!IMPORTANT]
> This is important information that should not be missed.

> [!WARNING]
> This is a warning about potential issues.

> [!CAUTION]
> This is a caution about dangerous operations.
```

### Supported Types

| Type        | Purpose                                      | Color  |
|-------------|----------------------------------------------|--------|
| `NOTE`      | Useful information the reader should know     | Blue   |
| `TIP`       | Helpful advice for better results             | Green  |
| `IMPORTANT` | Key information that should not be overlooked | Purple |
| `WARNING`   | Potential issues or things to be aware of     | Orange |
| `CAUTION`   | Dangerous operations or critical warnings     | Red    |

### Multi-Line Content

Admonitions can contain multiple lines and paragraphs:

```markdown
> [!WARNING]
> This is the first paragraph of the warning.
>
> This is a second paragraph with more details.
```

### How It Works

The admonition feature is implemented as a post-processor that transforms the rendered HTML output. When goldmark renders the markdown, blockquotes starting with `[!TYPE]` are converted from standard `<blockquote>` elements into styled `<div class="admonition">` elements.

The type markers are case-insensitive: `[!NOTE]`, `[!note]`, and `[!Note]` all produce the same result.

Regular blockquotes (those without a type marker) are not affected and render normally.

### Rendered Output

Each admonition is rendered as:

```html
<div class="admonition admonition-{type}">
  <p class="admonition-title">{Type}</p>
  <p>Content here</p>
</div>
```

The default theme includes CSS styles for all five admonition types with appropriate colors and styling that works in both light and dark modes.
