package util

import (
	"fmt"
	"testing"

	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "unrelated error",
			err:  fmt.Errorf("something else"),
			want: false,
		},
		{
			name: "ErrDefault404",
			err:  golangsdk.ErrDefault404{},
			want: true,
		},
		{
			name: "ErrUnexpectedResponseCode with 404",
			err:  golangsdk.ErrUnexpectedResponseCode{Actual: 404},
			want: true,
		},
		{
			name: "ErrUnexpectedResponseCode with 500",
			err:  golangsdk.ErrUnexpectedResponseCode{Actual: 500},
			want: false,
		},
		{
			name: "ErrDefault409 is not 404",
			err:  golangsdk.ErrDefault409{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "unrelated error",
			err:  fmt.Errorf("something else"),
			want: false,
		},
		{
			name: "ErrDefault409",
			err:  golangsdk.ErrDefault409{},
			want: true,
		},
		{
			name: "ErrUnexpectedResponseCode with 409",
			err:  golangsdk.ErrUnexpectedResponseCode{Actual: 409},
			want: true,
		},
		{
			name: "ErrUnexpectedResponseCode with 500",
			err:  golangsdk.ErrUnexpectedResponseCode{Actual: 500},
			want: false,
		},
		{
			name: "ErrDefault404 is not 409",
			err:  golangsdk.ErrDefault404{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConflict(tt.err); got != tt.want {
				t.Errorf("IsConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBadRequest(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "unrelated error",
			err:  fmt.Errorf("something else"),
			want: false,
		},
		{
			name: "ErrDefault400",
			err:  golangsdk.ErrDefault400{},
			want: true,
		},
		{
			name: "ErrUnexpectedResponseCode with 400",
			err:  golangsdk.ErrUnexpectedResponseCode{Actual: 400},
			want: true,
		},
		{
			name: "ErrUnexpectedResponseCode with 500",
			err:  golangsdk.ErrUnexpectedResponseCode{Actual: 500},
			want: false,
		},
		{
			name: "ErrDefault404 is not 400",
			err:  golangsdk.ErrDefault404{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBadRequest(tt.err); got != tt.want {
				t.Errorf("IsBadRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsServerError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "unrelated error",
			err:  fmt.Errorf("something else"),
			want: false,
		},
		{
			name: "ErrDefault500",
			err:  golangsdk.ErrDefault500{},
			want: true,
		},
		{
			name: "ErrUnexpectedResponseCode with 500",
			err:  golangsdk.ErrUnexpectedResponseCode{Actual: 500},
			want: true,
		},
		{
			name: "ErrUnexpectedResponseCode with 400",
			err:  golangsdk.ErrUnexpectedResponseCode{Actual: 400},
			want: false,
		},
		{
			name: "ErrDefault409 is not 500",
			err:  golangsdk.ErrDefault409{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsServerError(tt.err); got != tt.want {
				t.Errorf("IsServerError() = %v, want %v", got, tt.want)
			}
		})
	}
}
