package fuse

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"time"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/awnumar/memguard"
	"golang.org/x/crypto/ssh"

	"git.backbone/corpix/gpgfs/pkg/errors"
)

type (
	Enclave      = memguard.Enclave
	LockedBuffer = memguard.LockedBuffer
	PlainMessage = pgpcrypto.PlainMessage
	PGPMessage   = pgpcrypto.PGPMessage
	KeyFormat    = string
	KeyType      = string
	KeyCtor      = func(keyUID *packet.UserId, keyType KeyType, rawKey []byte) ([]byte, error)
	KeyCtors     = map[KeyFormat]KeyCtor
)

const (
	KeyFormatSSH KeyFormat = "ssh"

	KeyTypePrivate KeyType = "private"
	KeyTypePublic  KeyType = "public"
)

var (
	// see init()
	DefaultKeyUID *packet.UserId

	KeyFormatCtor = KeyCtors{
		KeyFormatSSH: NewKeyFromSSH,
	}

	NewEnclave      = memguard.NewEnclave
	NewPlainMessage = pgpcrypto.NewPlainMessage
	NewPGPMessage   = pgpcrypto.NewPGPMessage

	WipeBytes = memguard.WipeBytes
)

func init() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	DefaultKeyUID = packet.NewUserId(
		"root",
		"gpgfs fuse key",
		"root@"+hostname,
	)
}

func NewKeyFromSSH(keyUID *packet.UserId, keyType KeyType, rawPrivateKey []byte) ([]byte, error) {
	// NOTE: we only work with private keys as an input here
	// keyType is a key type to return, not the input key type

	var armorBlockType string
	switch keyType {
	case KeyTypePrivate:
		armorBlockType = openpgp.PrivateKeyType
	case KeyTypePublic:
		armorBlockType = openpgp.PublicKeyType
	default:
		return nil, errors.Errorf("failed to create key type from %q", keyType)
	}

	var (
		timeNull   = time.Unix(0, 0)
		pubKeyAlgo packet.PublicKeyAlgorithm
		primaryKey *packet.PublicKey
		privateKey *packet.PrivateKey
	)

	//

	key, err := ssh.ParseRawPrivateKey(rawPrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse private ssh key")
	}

	switch k := key.(type) {
	case *rsa.PrivateKey:
		pubKeyAlgo = packet.PubKeyAlgoRSA
		primaryKey = packet.NewRSAPublicKey(timeNull, &k.PublicKey)
		privateKey = packet.NewRSAPrivateKey(timeNull, k)
	case *ed25519.PrivateKey:
		pub := k.Public().(ed25519.PublicKey)
		pubKeyAlgo = packet.PubKeyAlgoEdDSA
		primaryKey = packet.NewEdDSAPublicKey(timeNull, &pub)
		privateKey = packet.NewEdDSAPrivateKey(timeNull, k)
	default:
		return nil, errors.Errorf("unsupported private key %T", key)
	}

	//

	gpgKey := &openpgp.Entity{
		PrimaryKey: primaryKey,
		PrivateKey: privateKey,
		Identities: make(map[string]*openpgp.Identity),
	}

	isPrimaryID := true
	gpgKey.Identities[keyUID.Id] = &openpgp.Identity{
		Name:   keyUID.Id,
		UserId: keyUID,
		SelfSignature: &packet.Signature{
			CreationTime:              timeNull,
			SigType:                   packet.SigTypePositiveCert,
			PubKeyAlgo:                pubKeyAlgo,
			Hash:                      crypto.SHA256, // FIXME: unhardcode?
			IsPrimaryId:               &isPrimaryID,
			FlagsValid:                true,
			FlagSign:                  true,
			FlagCertify:               true,
			FlagEncryptStorage:        true,
			FlagEncryptCommunications: true,
			IssuerKeyId:               &gpgKey.PrimaryKey.KeyId,
		},
	}

	err = gpgKey.Identities[keyUID.Id].SelfSignature.SignUserId(
		keyUID.Id,
		gpgKey.PrimaryKey,
		gpgKey.PrivateKey,
		nil,
	)
	if err != nil {
		return nil, err
	}
	gpgKey.Identities[keyUID.Id].Signatures = append(
		gpgKey.Identities[keyUID.Id].Signatures,
		gpgKey.Identities[keyUID.Id].SelfSignature,
	)

	//

	buf := bytes.NewBuffer(nil)
	writer, err := armor.Encode(buf, armorBlockType, make(map[string]string))
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode armor writer")
	}

	if keyType == KeyTypePrivate {
		err = gpgKey.SerializePrivate(writer, nil)
	} else {
		err = gpgKey.Serialize(writer)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to serialize key %q", keyType)
	}

	// NOTE: should be closed here (not defered)
	// because it flushes the output
	_ = writer.Close()

	return buf.Bytes(), nil
}

//

func NewKey(format KeyFormat, keyUID *packet.UserId, keyType KeyType, rawKey []byte) (*Enclave, error) {
	keyCtor, ok := KeyFormatCtor[format]
	if !ok {
		return nil, errors.Errorf(
			"unsupported key format %q for %q key type",
			format, keyType,
		)
	}

	buf, err := keyCtor(keyUID, keyType, rawKey)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to construct key from format %q for %q key type",
			format, keyType,
		)
	}

	return NewEnclave(buf), nil
}

//

func Encrypt(keyBuf *LockedBuffer, message *PlainMessage) ([]byte, error) {
	public, err := pgpcrypto.NewKeyFromArmored(keyBuf.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse public key")
	}

	if public.IsPrivate() {
		public, err = public.ToPublic()
		if err != nil {
			return nil, errors.Wrap(err, "failed to extract public key from private key")
		}
	}

	publicKeyRing, err := pgpcrypto.NewKeyRing(public)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new keyring")
	}

	cipherText, err := publicKeyRing.Encrypt(message, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encrypt message")
	}

	return cipherText.Data, nil
}

func Decrypt(keyBuf *LockedBuffer, encBuf []byte) (*PlainMessage, error) {
	private, err := pgpcrypto.NewKeyFromArmored(string(keyBuf.Bytes()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse the private key")
	}
	defer private.ClearPrivateParams()

	privateKeyRing, err := pgpcrypto.NewKeyRing(private)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the private key ring")
	}

	plainMessage, err := privateKeyRing.Decrypt(
		NewPGPMessage(encBuf),
		nil, 0,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt message")
	}

	return plainMessage, nil
}
