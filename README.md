# Image service

An example s3 image uploading microservice using the go aws sdk

## Usage

Runs on `http://localhost:8000`

### Setup

Create a `.env` file and add these four fields

```
AWS_ACCESS_KEY=
AWS_SECRET_KEY=
AWS_S3_BUCKET_NAME=
AWS_REGION=
```

Send a `multipart/form-data` request to `/upload` with a single field labeled `file`.
The service will upload the image to `images/<random_hex_number>.<file_ext>` in your bucket.
The file_ext will be inferred using the image MIME type byte pattern with `http.DetectContentType`
