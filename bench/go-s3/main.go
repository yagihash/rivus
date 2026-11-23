package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var client *s3.Client

func get(w http.ResponseWriter, r *http.Request) {
	out, err := client.GetObject(r.Context(), &s3.GetObjectInput{
		Bucket: aws.String("demo-bucket"),
		Key:    aws.String(r.URL.Query().Get("key")),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer out.Body.Close()
	io.Copy(w, out.Body)
}

func put(w http.ResponseWriter, r *http.Request) {
	_, err := client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket: aws.String("demo-bucket"),
		Key:    aws.String(r.URL.Query().Get("key")),
		Body:   r.Body,
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func list(w http.ResponseWriter, r *http.Request) {
	out, err := client.ListObjectsV2(r.Context(), &s3.ListObjectsV2Input{
		Bucket: aws.String("demo-bucket"),
		Prefix: aws.String(r.URL.Query().Get("prefix")),
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	for _, o := range out.Contents {
		io.WriteString(w, aws.ToString(o.Key)+"\n")
	}
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	client = s3.NewFromConfig(cfg)
	http.HandleFunc("/get", get)
	http.HandleFunc("/put", put)
	http.HandleFunc("/list", list)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
