// This file implements AWS Secrets Manager retrieval behind the fetcher interface.
// It depends on AWS SDK v2 and accepts injectable clients for isolated tests.
package secrets

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"golang.org/x/sync/singleflight"
)

// AWSSecretsFetcher is a production SecretFetcher that uses AWS Secrets Manager.
// It caches secret payloads in-memory for a short TTL to avoid repeated network calls.
type AWSSecretsFetcher struct {
	client  secretsManagerAPI
	clients sync.Map // map[string]secretsManagerAPI
	cache   sync.Map // map[string]cachedSecret
	baseCfg aws.Config
	group   singleflight.Group
}

type cachedSecret struct {
	val string
	ts  time.Time
}

// secretsManagerAPI describes the subset of the AWS SDK client used by this package.
type secretsManagerAPI interface {
	GetSecretValue(ctx context.Context, params *sm.GetSecretValueInput, optFns ...func(*sm.Options)) (*sm.GetSecretValueOutput, error)
}

// DefaultSecretCacheTTL controls how long secret values are cached in memory.
var DefaultSecretCacheTTL = 5 * time.Minute

// NewAWSSecretsFetcher loads AWS SDK configuration and returns a fetcher ready to query Secrets Manager.
func NewAWSSecretsFetcher(ctx context.Context) (*AWSSecretsFetcher, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	client := sm.NewFromConfig(cfg)
	return &AWSSecretsFetcher{client: client, baseCfg: cfg}, nil
}

// GetSecret fetches the secret payload identified by ref.SecretID.
// Results are cached for a short duration to avoid repeated calls during batch operations.
func (a *AWSSecretsFetcher) GetSecret(ctx context.Context, ref *SecretRef) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("nil secret ref")
	}

	// Select client by region when provided.
	var clientToUse secretsManagerAPI
	if ref.Region != "" {
		if v, ok := a.clients.Load(ref.Region); ok {
			clientToUse = v.(secretsManagerAPI)
		} else {
			rcfg := a.baseCfg
			rcfg.Region = ref.Region
			rc := sm.NewFromConfig(rcfg)
			actual, _ := a.clients.LoadOrStore(ref.Region, rc)
			clientToUse = actual.(secretsManagerAPI)
		}
	} else {
		clientToUse = a.client
	}

	cacheKey := ref.SecretID
	if ref.Region != "" {
		cacheKey = ref.Region + "|" + ref.SecretID
	}
	if v, ok := a.cache.Load(cacheKey); ok {
		cs := v.(cachedSecret)
		if time.Since(cs.ts) < DefaultSecretCacheTTL {
			return cs.val, nil
		}
	}

	// Use singleflight to dedupe concurrent fetches for the same secret.
	v, err, _ := a.group.Do(cacheKey, func() (interface{}, error) {
		in := &sm.GetSecretValueInput{SecretId: &ref.SecretID}
		resp, err := clientToUse.GetSecretValue(ctx, in)
		if err != nil {
			return nil, err
		}
		if resp.SecretString != nil {
			s := *resp.SecretString
			a.cache.Store(cacheKey, cachedSecret{val: s, ts: time.Now()})
			return s, nil
		}
		if resp.SecretBinary != nil {
			s := string(resp.SecretBinary)
			a.cache.Store(cacheKey, cachedSecret{val: s, ts: time.Now()})
			return s, nil
		}
		return nil, fmt.Errorf("secret value empty for %s", ref.SecretID)
	})
	if err != nil {
		return "", err
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("unexpected type from singleflight for %s", ref.SecretID)
}
