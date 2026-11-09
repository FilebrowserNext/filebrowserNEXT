package bolt

import (
	"errors"

	"github.com/asdine/storm/v3"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

type mfaRecord struct {
	UserID     uint   `storm:"id"`
	Ciphertext []byte `json:"ciphertext"`
}

type mfaBackend struct{ db *storm.DB }

func (s mfaBackend) Get(userID uint) ([]byte, error) {
	record := &mfaRecord{}
	if err := s.db.One("UserID", userID, record); err != nil {
		if errors.Is(err, storm.ErrNotFound) {
			return nil, fberrors.ErrNotExist
		}
		return nil, err
	}
	return record.Ciphertext, nil
}

func (s mfaBackend) Save(userID uint, ciphertext []byte) error {
	return s.db.Save(&mfaRecord{UserID: userID, Ciphertext: ciphertext})
}

func (s mfaBackend) Delete(userID uint) error {
	return s.db.DeleteStruct(&mfaRecord{UserID: userID})
}
