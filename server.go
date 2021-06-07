// Copyright (c) 2021 Satvik Reddy
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
)

//
// Configuration and Setup
//
type Config struct {
	AccessKey    string
	SecretKey    string
	S3bucketName string
	Region       string
}

type DeleteReq struct {
	URL string `json:"url"`
}

var ValidFiles = map[string]string{
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/png":  ".png",
	"image/jpeg": ".jpeg",
}

var uploader *s3manager.Uploader
var deleter *s3manager.BatchDelete
var config Config

func loadConfig() Config {
	AccessKey := os.Getenv("AWS_ACCESS_KEY")
	SecretKey := os.Getenv("AWS_SECRET_KEY")
	S3bucketName := os.Getenv("AWS_S3_BUCKET_NAME")
	Region := os.Getenv("AWS_REGION")

	return Config{
		AccessKey,
		SecretKey,
		S3bucketName,
		Region,
	}
}

//
// Utility functions used in our routes
//
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

	var fileKey strings.Builder

	fileKey.WriteString("images/")
	fileKey.WriteString(filename)
	fileKey.WriteString(ext)

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(config.S3bucketName),
		Key:    aws.String(fileKey.String()),
		Body:   bytes.NewReader(fileBuf.Bytes()),
	})

	if err != nil {
		return "", err
	}

	return aws.StringValue(&result.Location), nil
}

func DeleteObjectFromKey(key string) error {

	objects := []s3manager.BatchDeleteObject{
		{
			Object: &s3.DeleteObjectInput{
				Key:    aws.String(key),
				Bucket: aws.String(config.S3bucketName),
			},
		},
	}

	err := deleter.Delete(aws.BackgroundContext(), &s3manager.DeleteObjectsIterator{
		Objects: objects,
	})

	return err
}

//
// Routes
//
func RDelete(c *gin.Context) {
	key := c.Param("key")

	err := DeleteObjectFromKey(key)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("%s deleted successfully", key),
	})
}

func RUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	URL, err := UploadFromFileHeader(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
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

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	r.Use(cors.New(corsConfig))

	r.POST("/upload", RUpload)
	r.DELETE("/delete/:key", RDelete)

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
	deleter = s3manager.NewBatchDelete(sess)

	r.Run(":8000")
}
