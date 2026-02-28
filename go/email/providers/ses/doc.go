// Package ses provides an AWS SES v2 sender implementation built on the
// aws-sdk-go-v2 client.
//
// It constructs SES Simple content (text + HTML) from sender.Message and performs
// a single synchronous SendEmail call per Send invocation.
package ses
