package tester

import "testing"

func TestNameWithGVK_Match(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		query NameWithGVK
		given NameWithGVK
		want  bool
	}{
		{
			name: "ok: match using required fields only",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: true,
		},
		{
			name: "ok: match using required fields and namespace",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo", Namespace: "bar"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo", Namespace: "bar"},
			},
			want: true,
		},
		{
			name: "ok: match using required fields and group",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: true,
		},
		{
			name: "ok: match using required fields and version",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: true,
		},
		{
			name: "ok: match using required fields and groupVersion",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: true,
		},
		{
			name: "ok: match using required fields, namespace, and groupVersion",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo", Namespace: "bar"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo", Namespace: "bar"},
			},
			want: true,
		},
		{
			name: "ok: match using required fields even when namespace is defined",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo", Namespace: "bar"},
			},
			want: true,
		},
		{
			name: "ok: match using required fields even when group is defined",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: true,
		},
		{
			name: "ok: match using required fields even when version is defined",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: true,
		},
		{
			name: "err: not match when name is missed",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: false,
		},
		{
			name: "err: not match when kind is missed",
			query: NameWithGVK{
				GVK:            GVK{},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: false,
		},
		{
			name: "err: not match when name is different",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "bar"},
			},
			want: false,
		},
		{
			name: "err: not match when kind is different",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Pod"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: false,
		},
		{
			name: "err: not match when namespace is defined on query only",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo", Namespace: "bar"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: false,
		},
		{
			name: "err: not match when namespace is different",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo", Namespace: "bar"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo", Namespace: "baz"},
			},
			want: false,
		},
		{
			name: "err: not match when group is defined on query only",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: false,
		},
		{
			name: "err: not match when group is different",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "extensions"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: false,
		},
		{
			name: "err: not match when version is defined on query only",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: false,
		},
		{
			name: "err: not match when version is different",
			query: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps", Version: "v1"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			given: NameWithGVK{
				GVK:            GVK{Kind: "Deployment", Group: "apps", Version: "v2"},
				NamespacedName: NamespacedName{Name: "foo"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.query.Match(tt.given); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

}
