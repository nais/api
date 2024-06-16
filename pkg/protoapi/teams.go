package protoapi

func (t *Team) IsDeleted() bool {
	deletedAt := t.GetDeletedAt()
	return deletedAt != nil && !deletedAt.AsTime().IsZero()
}
