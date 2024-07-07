package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"time"
)

func Generator_key() {
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		err := os.Mkdir("data", 0755)
		if err != nil {
			log.Println("Error to create cert folder:", err)
		}
	}
	_, err1 := os.Stat("data/private.key")
	_, err2 := os.Stat("data/certificate.crt")
	if os.IsNotExist(err1) || os.IsNotExist(err2) {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			log.Fatal(err)
		}
		template := x509.Certificate{
			SerialNumber:          big.NewInt(1),
			NotBefore:             time.Now(),
			NotAfter:              time.Now().AddDate(10, 0, 0), // 有效期为十年
			BasicConstraintsValid: true,
		}
		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
		if err != nil {
			log.Fatal(err)
		}
		err = ioutil.WriteFile("data/private.key", encodePrivateKeyToPEM(privateKey), 0600)
		if err != nil {
			log.Fatal(err)
		}
		err = ioutil.WriteFile("data/certificate.crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	return pem.EncodeToMemory(privateKeyPEM)
}
