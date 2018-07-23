package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"strings"
	"time"

	"context"

	"cesanta.com/common/go/mgrpc/codec"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/watson"
	"github.com/cesanta/errors"
)

var (
	azureConnectionString = ""
	certFile              = ""
	keyFile               = ""
	caFile                = ""
)

func init() {
	flag.StringVar(&azureConnectionString, "azure-connection-string", "", "Azure connection string")
	flag.StringVar(&certFile, "cert-file", "", "Certificate file name")
	flag.StringVar(&keyFile, "key-file", "", "Key file name")
	flag.StringVar(&caFile, "ca-cert-file", "", "CA cert for TLS server verification")
	hiddenFlags = append(hiddenFlags, "ca-cert-file")
}

func createDevConn(ctx context.Context) (*dev.DevConn, error) {
	return createDevConnWithJunkHandler(ctx, func(junk []byte) {}, func(topic string, data []byte) {})
}

func createDevConnWithJunkHandler(
	ctx context.Context, junkHandler func(junk []byte), logHandler func(string, []byte),
) (*dev.DevConn, error) {
	port, err := getPort()
	if err != nil {
		return nil, errors.Trace(err)
	}
	c := dev.Client{Port: port, Timeout: *timeout, Reconnect: *reconnect}
	prefix := "serial://"
	if strings.Index(port, "://") > 0 {
		prefix = ""
	}
	addr := prefix + port

	// Init and pass TLS config if --cert-file and --key-file are specified
	var tlsConfig *tls.Config = nil
	if certFile != "" || strings.HasPrefix(port, "wss") || strings.HasPrefix(port, "https") || strings.HasPrefix(port, "mqtts") {

		tlsConfig = &tls.Config{
			InsecureSkipVerify: caFile == "",
		}

		// Load client cert / key if specified
		if certFile != "" && keyFile == "" {
			return nil, errors.Errorf("Please specify --key-file")
		}
		if certFile != "" {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, errors.Trace(err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		// Load CA cert if specified
		if caFile != "" {
			caCert, err := ioutil.ReadFile(caFile)
			if err != nil {
				return nil, errors.Trace(err)
			}
			tlsConfig.RootCAs = x509.NewCertPool()
			tlsConfig.RootCAs.AppendCertsFromPEM(caCert)
		}
	}

	codecOpts := &codec.Options{
		AzureDM: codec.AzureDMCodecOptions{
			ConnectionString: azureConnectionString,
		},
		MQTT: codec.MQTTCodecOptions{
			LogCallback: logHandler,
		},
		Serial: codec.SerialCodecOptions{
			BaudRate:             baudRateFlag,
			HardwareFlowControl:  hwFCFlag,
			JunkHandler:          junkHandler,
			SetControlLines:      setControlLines,
			InvertedControlLines: *invertedControlLines,
		},
		Watson: codec.WatsonCodecOptions{
			APIKey:       watson.WatsonAPIKeyFlag,
			APIAuthToken: watson.WatsonAPIAuthTokenFlag,
		},
	}
	// Due to lack of flow control, we send data in chunks and wait after each.
	// At higher, non-default bad rate we assume user knows what they are doing.
	if codecOpts.Serial.BaudRate <= 115200 {
		codecOpts.Serial.SendChunkSize = 16
		codecOpts.Serial.SendChunkDelay = 5 * time.Millisecond
	}
	devConn, err := c.CreateDevConnWithOpts(ctx, addr, *reconnect, tlsConfig, codecOpts)
	return devConn, errors.Trace(err)
}
