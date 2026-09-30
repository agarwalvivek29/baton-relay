module github.com/agarwalvivek29/baton-relay

go 1.22

require github.com/agarwalvivek29/baton v0.0.0

// Development only: build against the sibling checkout. Replaced by a tagged
// version before publishing.
replace github.com/agarwalvivek29/baton => ../baton
