package traefik

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

type (
	myStruct struct {
		Certificate string `json:"certificate"`
		Key         string `json:"key"`
	}
)

func GetCertFromTraefik(file, domain string) (cert tls.Certificate, err error) {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("error reading file: %v\n", err)
		return tls.Certificate{}, err
	}
	return GetCertifcate(string(data), domain)
}

func GetCertifcate(jsonData, domain string) (cert tls.Certificate, err error) {
	certData, keyData, err := getCertData(jsonData, domain)
	if err != nil {
		return tls.Certificate{}, err
	}
	decodedCertData, err := base64.StdEncoding.DecodeString(certData)
	if err != nil {
		return tls.Certificate{}, err
	}
	decodedKeyData, err := base64.StdEncoding.DecodeString(keyData)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(decodedCertData, decodedKeyData)
}

func getCertData(jsonData, domain string) (cert, key string, err error) {
	obj, err := oj.ParseString(jsonData)
	if err != nil {
		return "", "", err
	}

	jPath := fmt.Sprintf(`$..Certificates[?(@.domain.main == %q)]`, domain)
	path, err := jp.ParseString(jPath)
	if err != nil {
		return "", "", err
	}
	res := path.Get(obj)
	if res == nil {
		return "", "", fmt.Errorf("domain not found")
	}

	// we have the domain, put the data in a struct
	certData := myStruct{}
	err = oj.Unmarshal([]byte(oj.JSON(res[0])), &certData)
	if err != nil {
		return "", "", err
	}
	return certData.Certificate, certData.Key, nil
}
