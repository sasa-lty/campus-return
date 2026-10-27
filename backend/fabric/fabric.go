package fabric

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	Organization string
	MSPID        string
	PeerEndpoint string
	GatewayPeer  string
	CertPath     string
	KeyPath      string
	TLSCertPath  string
	Channel      string
	Chaincode    string
}

type Client struct {
	Gateway    *client.Gateway
	Contract   *client.Contract
	Connection *grpc.ClientConn
	Config     Config
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ConfigForOrg(prefix, organization, domain, mspID, endpoint, peer string) Config {
	samples := env("FABRIC_SAMPLES_PATH", "../../fabric-samples")
	organizations := filepath.Join(samples, "test-network", "organizations")
	if root := os.Getenv("FABRIC_ROOT"); root != "" {
		organizations = filepath.Join(root, "organizations")
	}
	crypto := env("FABRIC_"+prefix+"_CRYPTO_PATH", filepath.Join(organizations, "peerOrganizations", domain))
	user := "User1@" + domain
	return Config{
		Organization: organization,
		MSPID:        env("FABRIC_"+prefix+"_MSP_ID", mspID),
		PeerEndpoint: env("FABRIC_"+prefix+"_PEER_ENDPOINT", endpoint),
		GatewayPeer:  env("FABRIC_"+prefix+"_GATEWAY_PEER", peer),
		CertPath:     env("FABRIC_"+prefix+"_CERT_PATH", filepath.Join(crypto, "users", user, "msp", "signcerts")),
		KeyPath:      env("FABRIC_"+prefix+"_KEY_PATH", filepath.Join(crypto, "users", user, "msp", "keystore")),
		TLSCertPath:  env("FABRIC_"+prefix+"_TLS_CERT_PATH", filepath.Join(crypto, "peers", peer, "tls", "ca.crt")),
		Channel:      env("CHANNEL_NAME", "mychannel"),
		Chaincode:    env("CHAINCODE_NAME", "lostfound"),
	}
}

func New(cfg Config) (*Client, error) {
	certificatePEM, err := os.ReadFile(cfg.TLSCertPath)
	if err != nil {
		return nil, fmt.Errorf("%s read TLS certificate: %w", cfg.Organization, err)
	}
	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	conn, err := grpc.NewClient(cfg.PeerEndpoint, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(certPool, cfg.GatewayPeer)))
	if err != nil {
		return nil, fmt.Errorf("%s create gRPC connection: %w", cfg.Organization, err)
	}
	signCertPEM, err := readFirstFile(cfg.CertPath)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%s read identity certificate: %w", cfg.Organization, err)
	}
	signCert, err := identity.CertificateFromPEM(signCertPEM)
	if err != nil {
		conn.Close()
		return nil, err
	}
	id, err := identity.NewX509Identity(cfg.MSPID, signCert)
	if err != nil {
		conn.Close()
		return nil, err
	}
	keyPEM, err := readFirstFile(cfg.KeyPath)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%s read private key: %w", cfg.Organization, err)
	}
	privateKey, err := identity.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		conn.Close()
		return nil, err
	}
	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		conn.Close()
		return nil, err
	}
	gateway, err := client.Connect(id, client.WithSign(sign), client.WithHash(hash.SHA256), client.WithClientConnection(conn), client.WithEvaluateTimeout(5*time.Second), client.WithEndorseTimeout(20*time.Second), client.WithSubmitTimeout(5*time.Second), client.WithCommitStatusTimeout(60*time.Second))
	if err != nil {
		conn.Close()
		return nil, err
	}
	contract := gateway.GetNetwork(cfg.Channel).GetContract(cfg.Chaincode)
	return &Client{Gateway: gateway, Contract: contract, Connection: conn, Config: cfg}, nil
}

func (c *Client) Close() {
	if c.Gateway != nil {
		c.Gateway.Close()
	}
	if c.Connection != nil {
		c.Connection.Close()
	}
}

// 依次完成提案、背书、提交与提交状态检查；有 TxID 不等于交易已成功。
func (c *Client) Submit(name string, args ...string) ([]byte, string, error) {
	proposal, err := c.Contract.NewProposal(name, client.WithArguments(args...))
	if err != nil {
		return nil, "", err
	}
	txID := proposal.TransactionID()
	transaction, err := proposal.Endorse()
	if err != nil {
		return nil, txID, err
	}
	result := transaction.Result()
	commit, err := transaction.Submit()
	if err != nil {
		return nil, txID, err
	}
	status, err := commit.Status()
	if err != nil {
		return nil, txID, err
	}
	if !status.Successful {
		return nil, txID, fmt.Errorf("transaction %s failed with code %d", txID, status.Code)
	}
	return result, txID, nil
}

func (c *Client) Evaluate(name string, args ...string) ([]byte, error) {
	return c.Contract.EvaluateTransaction(name, args...)
}

func readFirstFile(dir string) ([]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			return os.ReadFile(filepath.Join(dir, e.Name()))
		}
	}
	return nil, fmt.Errorf("no file in %s", dir)
}
