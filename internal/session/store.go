package session

import (
	"context"
	"encoding/json"

	"github.com/dynamost/telegram-bot/internal/models"
	"github.com/dynamost/telegram-bot/internal/service"
)

type Store struct {
	sessions service.SessionService
}

func NewStore(sessions service.SessionService) *Store {
	return &Store{sessions: sessions}
}

type persistedSession struct {
	Wizard json.RawMessage          `json:"wizard,omitempty"`
	Nav    []models.NavigationEntry `json:"nav,omitempty"`
}

func (s *Store) Load(ctx context.Context, adminID int64, wizardOut any, navOut *[]models.NavigationEntry) (*models.AdminSession, error) {
	session, err := s.sessions.Get(ctx, adminID)
	if err != nil || session == nil {
		if navOut != nil {
			*navOut = nil
		}
		return session, err
	}

	if navOut != nil {
		*navOut = nil
	}

	if len(session.FlowState) > 0 {
		var envelope persistedSession
		if err := json.Unmarshal(session.FlowState, &envelope); err == nil && (envelope.Wizard != nil || envelope.Nav != nil) {
			if wizardOut != nil && envelope.Wizard != nil {
				if err := json.Unmarshal(envelope.Wizard, wizardOut); err != nil {
					return nil, err
				}
			} else if wizardOut != nil && envelope.Wizard == nil {
				// keep zero value
			} else if wizardOut == nil && len(envelope.Wizard) > 0 {
				// ignore wizard payload when caller not interested
			} else if wizardOut != nil && envelope.Wizard == nil {
				// no wizard stored
			}
			if navOut != nil && len(envelope.Nav) > 0 {
				copied := make([]models.NavigationEntry, len(envelope.Nav))
				copy(copied, envelope.Nav)
				*navOut = copied
			}
			return session, nil
		}
		if wizardOut != nil {
			if err := json.Unmarshal(session.FlowState, wizardOut); err != nil {
				return nil, err
			}
		}
	}
	return session, nil
}

func (s *Store) Save(ctx context.Context, adminID int64, flowName *string, wizardState any, nav []models.NavigationEntry) error {
	var payload []byte
	if wizardState != nil || len(nav) > 0 {
		env := persistedSession{}
		if wizardState != nil {
			buf, err := json.Marshal(wizardState)
			if err != nil {
				return err
			}
			env.Wizard = buf
		}
		if len(nav) > 0 {
			copied := make([]models.NavigationEntry, len(nav))
			copy(copied, nav)
			env.Nav = copied
		}
		buf, err := json.Marshal(env)
		if err != nil {
			return err
		}
		payload = buf
	}
	return s.sessions.Save(ctx, models.AdminSession{
		AdminID:     adminID,
		CurrentFlow: flowName,
		FlowState:   payload,
	})
}

func (s *Store) Clear(ctx context.Context, adminID int64) error {
	return s.sessions.Delete(ctx, adminID)
}
