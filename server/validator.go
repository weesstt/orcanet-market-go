package main

import (
	"regexp"
	"strings"
	"errors"
	"time"
	"github.com/golang/protobuf/proto"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	pb "orcanet/market"
)

type OrcaValidator struct{}

/*
 * Given a list of values from the DHT, select index of the best one. This is determined by
 * checking which value is the longest and is valid according to the OrcaValidator. 
 * 
 * Parameters:
 *   key: SHA256 Hash String of file being registered
 *   value: A slice of byte slices that represent the values to be compared. 
 *
 * Returns:
 *   The index of the best value
 *   An error, if any
 * Author: Austin
 */
func (v OrcaValidator) Select(key string, value [][]byte) (int, error){
	max := 0
	maxIndex := 0
	latestTime := uint64(0);
	for i := 0; i < len(value); i++ {
		if len(value[i]) > max {
			//validate chain 
			if v.Validate(key, value[i]) == nil {
				suppliedTime := uint64(0)
				for j := 1; j < 5; j++ {
					suppliedTime = suppliedTime | uint64(value[i][len(value[i]) - j]) << (j - 1)
				}

				if(suppliedTime > latestTime){
					max = len(value[i])
					latestTime = suppliedTime;
					maxIndex = i;
				}
			}
		}
	}
	return maxIndex, nil;
}

//
//
//
/*
 * Validates keys and values that are being put into the OrcaNet market DHT.
 * Keys must conform to a SHA256 hash, Values must conform the specification in /server/README.md
 * 
 * Parameters:
 *   key: SHA256 Hash String of file being registered
 *   value: The value to be put into the DHT, must conform to specification in /server/README.md
 *
 * Returns:
 *   An error, if any
 * Author: Austin
 */
func (v OrcaValidator) Validate(key string, value []byte) error{
	// verify key is a sha256 hash
    hexPattern := "^[a-fA-F0-9]{64}$"
    regex := regexp.MustCompile(hexPattern)
    if !regex.MatchString(strings.Replace(key, "orcanet/market/", "", -1)) {
		return errors.New("Provided key is not in the form of a SHA-256 digest!")
	}

	pubKeySet := make(map[string] bool)

	for i := 0; i < len(value) - 4; i++ {
		messageLength := uint16(value[i + 1]) << 8 | uint16(value[i])
		digitalSignatureLength := uint16(value[i + 3]) << 8 | uint16(value[i + 2])
		contentLength := messageLength + digitalSignatureLength
		user := &pb.User{}

		err := proto.Unmarshal(value[i + 4:i + 4 + int(messageLength)], user) 
		if err != nil {
			return err
		}

		if pubKeySet[string(user.GetId())] == true {
			return errors.New("Duplicate record for the same public key found!")
		}else{
			pubKeySet[string(user.GetId())] = true
		}

		userMessageBytes := value[i + 4:i + 4 + int(messageLength)]

		publicKey, err := crypto.UnmarshalRsaPublicKey(user.GetId())
		if err != nil{
			return err
		}

		signatureBytes := value[i + 4 + int(messageLength):i + 4 + int(contentLength)]
		valid, err := publicKey.Verify(userMessageBytes, signatureBytes) //this function will automatically compute hash of data to compare signauture
		
		if err != nil {
			return err
		}

		if !valid {
			return errors.New("Signature invalid!")
		}

		i = i + 4 + int(contentLength) - 1
	}

    currentTime := time.Now().UTC()
    unixTimestamp := currentTime.Unix()
    unixTimestampInt64 := uint64(unixTimestamp)

	suppliedTime := uint64(0)
	for i := 1; i < 5; i++ {
		suppliedTime = suppliedTime | (uint64(value[len(value) - i]) << (i - 1))
	}

	if(suppliedTime > unixTimestampInt64){
		return errors.New("Supplied time cannot be less than current time")
	}

	return nil
}