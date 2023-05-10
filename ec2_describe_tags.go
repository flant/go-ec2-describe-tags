//ec2-describe-tags --filter "resource-type=instance" --filter "resource-id=$(ec2metadata --instance-id)"
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	awsTokenTTLHeader      = "X-aws-ec2-metadata-token-ttl-seconds"
	awsTokenHeader         = "X-aws-ec2-metadata-token"
	awsIMDSURL             = "http://169.254.169.254"
	awsIMDSv2TokenPath     = "latest/api/token"
	awsIMDSRegionPath      = "latest/meta-data/placement/region"
	awsIMDSInstanceIDPath  = "latest/meta-data/instance-id"
	awsIMDSv2TokenTTL      = "30"
)

//get imsdv2 token
func getToken() (string, error) {
	client := http.Client{}

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s", awsIMDSURL, awsIMDSv2TokenPath), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set(awsTokenTTLHeader, awsIMDSv2TokenTTL)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	token, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

func getMetadata(token, path string) (string, error) {
	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", awsIMDSURL, path), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set(awsTokenHeader, token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func main() {

	var awsAccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	var awsSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	var region = os.Getenv("AWS_REGION")
	var instanceID = os.Getenv("EC2_INSTANCE_ID")
	var pdelim = "\n"
	var kvdelim = "="
	var queryMetadata = false

	flag.StringVar(&awsAccessKey, "access_key", awsAccessKey, "AWS Access Key")
	flag.StringVar(&awsSecretAccessKey, "secret_access_key", awsSecretAccessKey, "AWS Secret Access Key")
	flag.StringVar(&region, "region", region, "AWS Region identifier")
	flag.StringVar(&instanceID, "instance_id", instanceID, "EC2 instance id")
	flag.StringVar(&pdelim, "p_delim", pdelim, "delimiter between key-value pairs")
	flag.StringVar(&kvdelim, "kv_delim", kvdelim, "delimiter between key and value")
	flag.BoolVar(&queryMetadata, "query_meta", queryMetadata, "query metadata service for instance_id and region")

	flag.Parse()

	if queryMetadata {
		token, err := getToken()
		if err != nil {
			fmt.Printf("Failed to get IMDSv2 token: %s", err)
			os.Exit(1)
		}

		if region == "" {
			region, err = getMetadata(token, awsIMDSRegionPath)
			if err != nil {
				fmt.Printf("%s", err)
				os.Exit(1)
			}
		}

		if instanceID == "" {
			instanceID, err = getMetadata(token, awsIMDSInstanceIDPath)
			if err != nil {
				fmt.Printf("%s", err)
				os.Exit(1)
			}
		}
	}

	var creds *credentials.Credentials
	if len(awsAccessKey) != 0 || len(awsSecretAccessKey) != 0 {
		creds = credentials.NewStaticCredentials(awsAccessKey, awsSecretAccessKey, "")
	}

	sess, err := session.NewSession(&aws.Config{Credentials: creds, Region: aws.String(region)})
	svc := ec2.New(sess, &aws.Config{Credentials: creds, Region: aws.String(region)})

	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-id"),
				Values: []*string{
					aws.String(instanceID),
				},
			},
		},
	}

	resp, err := svc.DescribeInstances(params)

	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}
	if len(resp.Reservations) == 0 {
		os.Exit(1)
	}
	for idx := range resp.Reservations {
		for _, inst := range resp.Reservations[idx].Instances {
			// https: //godoc.org/github.com/awslabs/aws-sdk-go/service/ec2#Instance
			var s []string
			for _, tag := range inst.Tags {
				s = append(s, *tag.Key+kvdelim+*tag.Value)
			}
			fmt.Println(strings.Join(s, pdelim))
		}
	}

}
