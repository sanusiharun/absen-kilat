package service

import (
	"context"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// ProfileService implements profile business logic on top of the user repo.
type ProfileService struct {
	users domain.UserRepository
}

// NewProfileService creates a ProfileService.
func NewProfileService(users domain.UserRepository) *ProfileService {
	return &ProfileService{users: users}
}

// ProfileUpdate carries optional fields to change on the profile.
type ProfileUpdate struct {
	Name                *string
	NotificationEnabled *bool
}

// Get returns the user with the given id.
func (s *ProfileService) Get(ctx context.Context, userID string) (*domain.User, error) {
	return s.users.FindByID(ctx, userID)
}

// Update applies the provided changes to a user's profile.
func (s *ProfileService) Update(ctx context.Context, userID string, upd ProfileUpdate) (*domain.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if upd.Name != nil {
		if *upd.Name == "" {
			return nil, pkgerrors.Validation("name", "cannot be empty")
		}
		user.Name = *upd.Name
	}
	if upd.NotificationEnabled != nil {
		user.NotificationEnabled = *upd.NotificationEnabled
	}
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// SetAvatar updates the user's avatar path.
func (s *ProfileService) SetAvatar(ctx context.Context, userID, avatarPath string) (*domain.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	user.AvatarPath = avatarPath
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
