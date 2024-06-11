package protoapi

func (t *Team) IsDeleted() bool {
	return t.GetDeletedAt() != nil
}
