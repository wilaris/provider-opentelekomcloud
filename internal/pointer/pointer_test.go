package pointer

import "testing"

func TestTo(t *testing.T) {
	v := 42
	p := To(v)
	if p == nil || *p != 42 {
		t.Errorf("To(42) = %v, want pointer to 42", p)
	}

	s := To("hello")
	if s == nil || *s != "hello" {
		t.Errorf("To(\"hello\") = %v, want pointer to \"hello\"", s)
	}
}

func TestDeref(t *testing.T) {
	tests := []struct {
		name string
		ptr  *string
		def  string
		want string
	}{
		{
			name: "non-nil pointer",
			ptr:  To("value"),
			def:  "default",
			want: "value",
		},
		{
			name: "nil pointer",
			ptr:  nil,
			def:  "default",
			want: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Deref(tt.ptr, tt.def)
			if got != tt.want {
				t.Errorf("Deref() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		a    *int
		b    *int
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "a nil b non-nil",
			a:    nil,
			b:    To(1),
			want: false,
		},
		{
			name: "a non-nil b nil",
			a:    To(1),
			b:    nil,
			want: false,
		},
		{
			name: "same value",
			a:    To(42),
			b:    To(42),
			want: true,
		},
		{
			name: "different values",
			a:    To(1),
			b:    To(2),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Equal(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}
