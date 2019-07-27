package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	fdk "github.com/fnproject/fdk-go"
	"github.com/jung-kurt/gofpdf"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/objectstorage"
)

func main() {
	fdk.Handle(fdk.HandlerFunc(text2PDF))
}

const privateKeyFolder string = "/function"

func text2PDF(ctx context.Context, in io.Reader, out io.Writer) {

	var evt OCIEvent
	json.NewDecoder(in).Decode(&evt)
	log.Println("Got OCI event", evt)
	log.Println("Got Casper details", evt.Data)

	fileName := evt.Data.ResourceName
	log.Println("File name", fileName)
	extension := strings.Split(fileName, ".")[1]

	if extension != "txt" {
		log.Println("File is not of type .txt - " + fileName)
		return
	}

	namespace := evt.Data.AdditionalDetails.Namespace
	bucketName := evt.Data.AdditionalDetails.BucketName

	log.Println("Storage Bucket namespace ", namespace)
	log.Println("Input storage Bucket name ", bucketName)

	fnCtx := fdk.GetContext(ctx)

	tenancy := fnCtx.Config()["TENANT_OCID"]
	user := fnCtx.Config()["USER_OCID"]
	region := fnCtx.Config()["REGION"]
	fingerprint := fnCtx.Config()["FINGERPRINT"]
	privateKeyName := fnCtx.Config()["PRIVATE_KEY_NAME"]
	privateKeyLocation := privateKeyFolder + "/" + privateKeyName
	passphrase := fnCtx.Config()["PASSPHRASE"]
	outputBucket := fnCtx.Config()["OUTPUT_BUCKET"]

	log.Println("TENANT_OCID ", tenancy)
	log.Println("USER_OCID ", user)
	log.Println("REGION ", region)
	log.Println("FINGERPRINT ", fingerprint)
	log.Println("PRIVATE_KEY_NAME ", privateKeyName)
	log.Println("PRIVATE_KEY_LOCATION ", privateKeyLocation)
	log.Println("OUTPUT_BUCKET ", outputBucket)

	privateKey, err := ioutil.ReadFile(privateKeyLocation)
	if err == nil {
		log.Println("read private key from ", privateKeyLocation)
	} else {
		resp := FailedResponse{Message: "Unable to read private Key", Error: err.Error()}
		log.Println(resp.toString())
		json.NewEncoder(out).Encode(resp)
		return
	}

	rawConfigProvider := common.NewRawConfigurationProvider(tenancy, user, region, fingerprint, string(privateKey), common.String(passphrase))
	osclient, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(rawConfigProvider)

	if err != nil {
		resp := FailedResponse{Message: "Problem getting Object Store Client handle", Error: err.Error()}
		log.Println(resp.toString())
		json.NewEncoder(out).Encode(resp)
		return
	}

	nameWithoutType := strings.Split(fileName, ".")[0]
	opFileName := nameWithoutType + ".pdf"
	tmpFileLocation := "/tmp/" + opFileName

	log.Println("Reading text file " + fileName + " from storage bucket " + bucketName)

	req := objectstorage.GetObjectRequest{NamespaceName: common.String(namespace), BucketName: common.String(bucketName), ObjectName: common.String(fileName)}
	resp, err := osclient.GetObject(context.Background(), req)

	if err != nil {
		resp := FailedResponse{Message: "Could not read file " + fileName + " from bucket " + bucketName, Error: err.Error()}
		log.Println(resp.toString())
		json.NewEncoder(out).Encode(resp)
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Content)

	text := buf.String()

	//log.Println("Got text from file", text)

	err = textToPDF(text, tmpFileLocation)
	if err == nil {
		log.Println("PDF Written to " + tmpFileLocation)
	} else {
		resp := FailedResponse{Message: "Failed to write PDF", Error: err.Error()}
		log.Println(resp.toString())
		json.NewEncoder(out).Encode(resp)
		return
	}

	defer func() {
		fileErr := os.Remove(tmpFileLocation)
		if fileErr == nil {
			log.Println("Deleted temp file", tmpFileLocation)
		} else {
			log.Println("Error removing temp file", fileErr.Error())
		}
	}()

	file, err := os.Open(tmpFileLocation)
	if err != nil {

		resp := FailedResponse{Message: "failed to read PDF from " + tmpFileLocation, Error: err.Error()}
		log.Println(resp.toString())
		json.NewEncoder(out).Encode(resp)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	//log.Println("PDF File size -- ", info.Size())

	putReq := objectstorage.PutObjectRequest{ContentLength: common.Int64(info.Size()), PutObjectBody: file, NamespaceName: common.String(namespace), BucketName: common.String(outputBucket), ObjectName: common.String(opFileName)}
	_, err = osclient.PutObject(context.Background(), putReq)

	if err == nil {
		msg := "PDF " + opFileName + " written to storage bucket - " + outputBucket
		log.Println(msg)
		out.Write([]byte(msg))
	} else {
		resp := FailedResponse{Message: "Failed to write PDF to bucket", Error: err.Error()}
		log.Println(resp.toString())
		json.NewEncoder(out).Encode(resp)
		return
	}
}

func textToPDF(text, tmpFileLocation string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Times", "", 12)
	pdf.MultiCell(0, 5, string(text), "", "", false)
	return pdf.OutputFileAndClose(tmpFileLocation)
}

//FailedResponse ...
type FailedResponse struct {
	Message string
	Error   string
}

func (response FailedResponse) toString() string {
	return response.Message + " due to " + response.Error
}

type OCIEvent struct {
	CloudEventsVersion string      `json:"cloudEventsVersion"`
	EventID            string      `json:"eventID"`
	EventType          string      `json:"eventType"`
	Source             string      `json:"source"`
	EventTypeVersion   string      `json:"eventTypeVersion"`
	EventTime          time.Time   `json:"eventTime"`
	SchemaURL          interface{} `json:"schemaURL"`
	ContentType        string      `json:"contentType"`
	Extensions         Extensions  `json:"extensions"`
	Data               Data        `json:"data"`
}
type Extensions struct {
	CompartmentID string `json:"compartmentId"`
}
type FreeFormTags struct {
}
type DefinedTags struct {
}
type AdditionalDetails struct {
	ETag          string      `json:"eTag"`
	Namespace     string      `json:"namespace"`
	ArchieveState interface{} `json:"archieveState"`
	BucketName    string      `json:"bucketName"`
	BucketID      string      `json:"bucketId"`
}
type Data struct {
	CompartmentID      string            `json:"compartmentId"`
	CompartmentName    string            `json:"compartmentName"`
	ResourceName       string            `json:"resourceName"`
	ResourceID         string            `json:"resourceId"`
	AvailabilityDomain string            `json:"availabilityDomain"`
	FreeFormTags       FreeFormTags      `json:"freeFormTags"`
	DefinedTags        DefinedTags       `json:"definedTags"`
	AdditionalDetails  AdditionalDetails `json:"additionalDetails"`
}
