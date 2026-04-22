package renderer

import "testing"

func TestAddHeadingAnchors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "h1 with id",
			in:   `<h1 id="title">Title</h1>`,
			want: `<h1 id="title">Title <a href="#title" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h1>`,
		},
		{
			name: "h2 with id",
			in:   `<h2 id="section">Section</h2>`,
			want: `<h2 id="section">Section <a href="#section" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h2>`,
		},
		{
			name: "h3 with id",
			in:   `<h3 id="sub">Sub</h3>`,
			want: `<h3 id="sub">Sub <a href="#sub" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h3>`,
		},
		{
			name: "h4 with id",
			in:   `<h4 id="deep">Deep</h4>`,
			want: `<h4 id="deep">Deep <a href="#deep" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h4>`,
		},
		{
			name: "h5 with id",
			in:   `<h5 id="deeper">Deeper</h5>`,
			want: `<h5 id="deeper">Deeper <a href="#deeper" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h5>`,
		},
		{
			name: "h6 with id",
			in:   `<h6 id="deepest">Deepest</h6>`,
			want: `<h6 id="deepest">Deepest <a href="#deepest" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h6>`,
		},
		{
			name: "heading without id is not modified",
			in:   `<h2>No ID</h2>`,
			want: `<h2>No ID</h2>`,
		},
		{
			name: "anchor href matches heading id",
			in:   `<h2 id="my-section">My Section</h2>`,
			want: `<h2 id="my-section">My Section <a href="#my-section" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h2>`,
		},
		{
			name: "multiple headings in same content",
			in:   "<h1 id=\"first\">First</h1>\n<p>paragraph</p>\n<h2 id=\"second\">Second</h2>",
			want: "<h1 id=\"first\">First <a href=\"#first\" class=\"heading-anchor\" aria-hidden=\"true\" tabindex=\"-1\">#</a></h1>\n<p>paragraph</p>\n<h2 id=\"second\">Second <a href=\"#second\" class=\"heading-anchor\" aria-hidden=\"true\" tabindex=\"-1\">#</a></h2>",
		},
		{
			name: "id with special characters",
			in:   `<h2 id="hello-world_123">Hello World</h2>`,
			want: `<h2 id="hello-world_123">Hello World <a href="#hello-world_123" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h2>`,
		},
		{
			name: "heading with inline markup",
			in:   `<h2 id="code">Code <code>example</code></h2>`,
			want: `<h2 id="code">Code <code>example</code> <a href="#code" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a></h2>`,
		},
		{
			name: "empty content is unchanged",
			in:   "",
			want: "",
		},
		{
			name: "no headings at all",
			in:   "<p>Just a paragraph.</p>",
			want: "<p>Just a paragraph.</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(addHeadingAnchors([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("addHeadingAnchors()\ngot:  %s\nwant: %s", got, tt.want)
			}
		})
	}
}
