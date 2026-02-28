package ses

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/denisvmedia/inventario/email/sender"
)

// Config defines AWS SES sender settings.
//
// Either provide a preconfigured SDK Client (for dependency injection/testing)
// or a Region so New can build one from default AWS credentials/config sources.
type Config struct {
	// Region is required when Client is nil.
	Region string
	// Client allows caller-managed SDK configuration and test doubles.
	Client *sesv2.Client
}
type sesAPI interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// Sender delivers email through AWS SES v2.
//
// It maps sender.Message to SES "Simple" content with both text and HTML parts.
type Sender struct {
	client sesAPI
}

// New creates an SES sender from config.
//
// When Client is nil, it initializes a new SDK client from ambient AWS config.
func New(cfg Config) (*Sender, error) {
	if cfg.Client != nil {
		return &Sender{client: cfg.Client}, nil
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		return nil, fmt.Errorf("ses provider requires AWS_REGION")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &Sender{
		client: sesv2.NewFromConfig(awsCfg),
	}, nil
}

// Send performs one SES SendEmail request.
//
// Errors are returned directly so upstream orchestration can decide retry policy.
func (s *Sender) Send(ctx context.Context, message sender.Message) error {
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(message.From),
		Destination: &sesv2types.Destination{
			ToAddresses: []string{message.To},
		},
		Content: &sesv2types.EmailContent{
			Simple: &sesv2types.Message{
				Subject: &sesv2types.Content{
					Data:    aws.String(message.Subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &sesv2types.Body{
					Html: &sesv2types.Content{
						Data:    aws.String(message.HTML),
						Charset: aws.String("UTF-8"),
					},
					Text: &sesv2types.Content{
						Data:    aws.String(message.Text),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}
	if strings.TrimSpace(message.ReplyTo) != "" {
		input.ReplyToAddresses = []string{message.ReplyTo}
	}
	if _, err := s.client.SendEmail(ctx, input); err != nil {
		return fmt.Errorf("ses send email: %w", err)
	}
	return nil
}
