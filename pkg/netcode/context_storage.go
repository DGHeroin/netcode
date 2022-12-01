package netcode

import (
    bolt "go.etcd.io/bbolt"
)

func (c *Context) StorageOpen(filename string) interface{} {
    db, err := bolt.Open(filename, 0600, nil)
    if err != nil {
        return nil
    }
    return db
}
func (c *Context) StorageGet(a interface{}, bucket, key string) ([]byte, error) {
    db, ok := a.(*bolt.DB)
    if !ok {
        return nil, bolt.ErrInvalid
    }
    var (
        result []byte
        err    error
    )
    err = db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte(bucket))
        if b == nil {
            return bolt.ErrBucketNotFound
        }
        result = b.Get([]byte(key))
        return nil
    })
    return result, err
}
func (c *Context) StorageSet(a interface{}, bucket, key string, value []byte) error {
    db, ok := a.(*bolt.DB)
    if !ok {
        return bolt.ErrInvalid
    }
    var (
        err error
    )
    err = db.Update(func(tx *bolt.Tx) error {
        b, err := tx.CreateBucketIfNotExists([]byte(bucket))
        if err != nil {
            return err
        }
        if b == nil {
            return bolt.ErrBucketNotFound
        }
        return b.Put([]byte(key), value)
    })
    return err
}

func (c *Context) StorageClose(a interface{}) error {
    db, ok := a.(*bolt.DB)
    if !ok {
        return bolt.ErrInvalid
    }
    return db.Close()
}
