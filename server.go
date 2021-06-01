// Copyright (c) 2021 Satvik Reddy
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	AccessKey    string
	SecretKey    string
	S3bucketName string
	Region       string
}

var ValidFiles = map[string]string{
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/png":  ".png",
	"image/jpeg": ".jpeg",
}

var uploader *s3manager.Uploader
var config Config

func loadConfig() Config {
	return Config{
		os.Getenv("AWS_ACCESS_KEY"),
		os.Getenv("AWS_SECRET_KEY"),
		os.Getenv("AWS_S3_BUCKET_NAME"),
		os.Getenv("AWS_REGION"),
	}
}

func genFileName() (string, error) {
	buff := make([]byte, 32)
	_, err := rand.Read(buff)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(buff), nil
}

func getFileType(r io.Reader) (string, error) {
	buff := make([]byte, 512)
	_, err := r.Read(buff)
	if err != nil {
		return "", err
	}
	ftype := http.DetectContentType(buff)

	return ftype, nil
}

func UploadFromFileHeader(header *multipart.FileHeader) (string, error) {
	file, err := header.Open()
	if err != nil {
		return "", err
	}

	filename, err := genFileName()
	if err != nil {
		return "", err
	}

	var fileBuf bytes.Buffer
	fileTee := io.TeeReader(file, &fileBuf)

	ftype, err := getFileType(fileTee)
	if err != nil {
		return "", err
	}

	ioutil.ReadAll(fileTee)

	ext, ok := ValidFiles[ftype]

	if !ok {
		return "", errors.New("not a valid file")
	}

	var filePathBuff bytes.Buffer

	filePathBuff.WriteString("images/")
	filePathBuff.WriteString(filename)
	filePathBuff.WriteString(ext)

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(config.S3bucketName),
		Key:    aws.String(filePathBuff.String()),
		Body:   bytes.NewReader(fileBuf.Bytes()),
	})

	if err != nil {
		return "", err
	}

	return aws.StringValue(&result.Location), nil
}

func RUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file not found",
		})
		return
	}

	URL, err := UploadFromFileHeader(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "file couldn't be uploaded",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"url": URL,
	})
}

func main() {
	r := gin.Default()
	r.MaxMultipartMemory = 5 << 20

	r.POST("/upload", RUpload)

	config = loadConfig()

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(config.Region),
		Credentials: credentials.NewStaticCredentials(
			config.AccessKey,
			config.SecretKey,
			"",
		),
	}))

	uploader = s3manager.NewUploader(sess)

	r.Run(":8000")
}
