package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	flagTargetBucket = flag.String("bucket", "", "Name to use for the newly created s3 website bucket")
	flagTemplate     = flag.String("template", os.Getenv("TEMPLATE_S3BUCKET"), "Bucket to use as template for content cloning")
	flagPublic       = flag.Bool("public", false, "Allow public read via s3 api")
	flagRegion       = flag.String("region", "us-east-1", "AWS Region for bucket and website")
	flagCleanup      = flag.Bool("cleanup", false, "Delete bucket and website")

	// use session from AWS_PROFILE or AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
	mySession = session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
)

func main() {
	// create aws session using all available methods, credentials, env vars, and profile

	flag.Parse()
	Region := *flagRegion
	BucketName := *flagTargetBucket
	objectACL := "public-read"
	objectContentType := "text/html"
	templateBucket := *flagTemplate
	// Initliaze Session
	svc := s3.New(mySession, aws.NewConfig().WithRegion(Region))

	if *flagCleanup {
		_, err := svc.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: &BucketName,
		})
		if err != nil {
			log.Fatal("Failed to delete bucket", err)
		} else {
			log.Println("deleted bucket", BucketName)
		}
	}
	// Create S3 Bucket
	_, err := svc.CreateBucket(&s3.CreateBucketInput{Bucket: &BucketName})
	if err != nil {
		log.Fatal("Failed to create bucket", err)
	} else {
		log.Println("Created new bucket", BucketName)
	}

	if *flagPublic {
		_, err = svc.PutBucketAcl(&s3.PutBucketAclInput{
			ACL:    &objectACL,
			Bucket: &BucketName,
		})
		if err != nil {
			log.Fatal("Failed to create bucket ACL", err)
		} else {
			log.Println("Set acl to public-read on", BucketName)
		}
	}
	// Put Bucket Website
	IndexDocument := "index.html"
	WebSiteConfig := &s3.WebsiteConfiguration{IndexDocument: &s3.IndexDocument{Suffix: &IndexDocument}}
	_, err = svc.PutBucketWebsite(&s3.PutBucketWebsiteInput{Bucket: &BucketName, WebsiteConfiguration: WebSiteConfig})
	if err != nil {
		log.Fatal("Failed to apply website configuration bucket ", err)
	} else {
		log.Println("Defined ", BucketName, " as an S3 website")
	}

	// Sync from template bucket - Get Content
	listBucketContents, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: &templateBucket})
	if err != nil {
		log.Fatal("Failed to read file list from bucket ", err)
	} else {
		log.Println("retrieved source content from", templateBucket)
	}
	bucketContents := listBucketContents.Contents
	for i := range bucketContents {
		fileName := bucketContents[i].Key
		if *fileName == string("index.html") {
			objectContentType = "text/html"

		} else {
			objectContentType = "image/x-icon"
		}
		getFileBytes, err := svc.GetObject(&s3.GetObjectInput{Bucket: &templateBucket, Key: fileName})
		if err != nil {
			log.Fatal("Failed to retrieve file from source bucket", err)
		} else {
			log.Println("Retrieved file", *fileName, "from", templateBucket)
		}
		templateFile := getFileBytes.Body
		fileBytes, err := ioutil.ReadAll(templateFile)
		fileContent := bytes.NewReader(fileBytes)
		_, err = svc.PutObject(&s3.PutObjectInput{Bucket: &BucketName, Body: fileContent, Key: fileName, ContentType: &objectContentType})
		if err != nil {
			log.Fatal("Failed to write object", fileName, "to bucket", BucketName, err)
		} else {
			log.Println("wrote file", *fileName, "to", BucketName)
		}
		_, err = svc.PutObjectAcl(&s3.PutObjectAclInput{Bucket: &BucketName, Key: fileName, ACL: &objectACL})
		if err != nil {
			log.Fatal("Failed to write ACL on", fileName, err)

		} else {
			log.Println("wrote public-read ACL on file", *fileName, "to", BucketName)
		}

	}
	if err == nil {
		log.Println("Bucket website created: http://" + BucketName + ".s3-website-" + Region + ".amazonaws.com")
	}
}
