package formatter

import "github.com/moby/moby/client/pkg/stringid"

type ContainerID struct {
	ID string
}

func (cid ContainerID) ShortID() string {
	return stringid.TruncateID(cid.ID)
}

func (cid ContainerID) LongID() string {
	return cid.ID
}

func (cid ContainerID) String() string {
	return cid.ShortID()
}
