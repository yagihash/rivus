package github

import "zoo/mark"

// This file is not generated (the generator never touches a file that has another name than the package's own): it has what the
// zoo needs and go-github does not have.

// NewClientFor makes a client whose calls return the marker of tag where go-github returns what a user of GitHub wrote. (Not in
// go-github: the real way is a server that answers.) The marker is the one of the case that runs: the generated values carry zooV.
func NewClientFor(tag string) *Client {
	zooV = mark.Src(tag)
	return &Client{}
}
