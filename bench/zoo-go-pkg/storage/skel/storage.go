// Package storage is a skeleton of cloud.google.com/go/storage v1.71.0 for the zoo (docs/DESIGN-package-models.md, 6.8): the
// public signatures of the calls that take the name of a bucket or of an object, and bodies that only record the name with zoo/mark
// where the built-in model (internal/model/builtin/go-storage.yaml) has a sink row, so that the run of a cell checks the row. It
// is not the library: the analysis does not read these bodies where the model has a row, and the real library is never run.
// NewClient takes no options here (the real one has `opts ...option.ClientOption`); a call without options is the same in both.
package storage

import (
	"context"

	"zoo/mark"
)

// Client is the client of the service.
type Client struct{}

// NewClient makes a client.
func NewClient(ctx context.Context) (*Client, error) { return &Client{}, nil }

// BucketHandle is the handle of a bucket.
type BucketHandle struct{ name string }

// ObjectHandle is the handle of an object.
type ObjectHandle struct{ bucket, name string }

// Bucket returns the handle of the bucket named name.
func (c *Client) Bucket(name string) *BucketHandle {
	mark.SinkCmd(name)
	return &BucketHandle{name: name}
}

// Object returns the handle of the object named name in the bucket.
func (b *BucketHandle) Object(name string) *ObjectHandle {
	mark.SinkCmd(name)
	return &ObjectHandle{bucket: b.name, name: name}
}

// BucketName returns the name of the bucket.
func (b *BucketHandle) BucketName() string { return b.name }

// ObjectName returns the name of the object.
func (o *ObjectHandle) ObjectName() string { return o.name }

// SignedURLOptions are the options of a signed URL.
type SignedURLOptions struct {
	GoogleAccessID string
	Method         string
}

// SignedURL returns a signed URL for the object.
func SignedURL(bucket, object string, opts *SignedURLOptions) (string, error) {
	mark.SinkCmd(bucket)
	mark.SinkCmd(object)
	return "", nil
}

// SignedURL returns a signed URL for the object of the bucket.
func (b *BucketHandle) SignedURL(object string, opts *SignedURLOptions) (string, error) {
	mark.SinkCmd(object)
	return "", nil
}

// PostPolicyV4Options are the options of a signed post policy.
type PostPolicyV4Options struct{ GoogleAccessID string }

// PostPolicyV4 is a signed post policy.
type PostPolicyV4 struct{ URL string }

// GenerateSignedPostPolicyV4 generates a post policy for the object of the bucket.
func (b *BucketHandle) GenerateSignedPostPolicyV4(object string, opts *PostPolicyV4Options) (*PostPolicyV4, error) {
	mark.SinkCmd(object)
	return &PostPolicyV4{}, nil
}

// Conditions are the preconditions of a call.
type Conditions struct{ GenerationMatch int64 }

// MoveObjectDestination provides the destination object name and the preconditions of Move.
type MoveObjectDestination struct {
	Object     string
	Conditions *Conditions
}

// ObjectAttrs are the attributes of an object.
type ObjectAttrs struct{ Name string }

// Move moves the object to the destination.
func (o *ObjectHandle) Move(ctx context.Context, destination MoveObjectDestination) (*ObjectAttrs, error) {
	mark.SinkCmd(destination.Object)
	return &ObjectAttrs{Name: destination.Object}, nil
}

// Query is a query of a listing.
type Query struct {
	Prefix      string
	StartOffset string
	EndOffset   string
}

// ObjectIterator iterates over the objects of a listing.
type ObjectIterator struct{}

// Next returns the next object.
func (it *ObjectIterator) Next() (*ObjectAttrs, error) { return nil, nil }

// Objects returns an iterator over the objects of the bucket.
func (b *BucketHandle) Objects(ctx context.Context, q *Query) *ObjectIterator {
	mark.SinkCmd(q.Prefix)
	return &ObjectIterator{}
}
