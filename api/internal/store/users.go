package store

import (
	"context"
	"fmt"
)

// User is the caller's identity plus the organizations they belong to, which is everything the
// authorization rules in this service need.
type User struct {
	ID     string
	Email  string
	Name   string
	OrgIDs []string
}

// InOrg reports whether the user belongs to orgID.
func (u User) InOrg(orgID string) bool {
	for _, id := range u.OrgIDs {
		if id == orgID {
			return true
		}
	}
	return false
}

// FindUser loads a user and their memberships in one round trip, returning ErrNotFound when no
// such user exists.
func (s *Store) FindUser(ctx context.Context, id string) (User, error) {
	const q = `
		select u.id, u.email, u.display_name,
		       coalesce(array_agg(m.org_id) filter (where m.org_id is not null), '{}') as org_ids
		from users u
		left join memberships m on m.user_id = u.id
		where u.id = $1
		group by u.id
	`
	var u User
	err := s.pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Email, &u.Name, &u.OrgIDs)
	if err != nil {
		return User{}, notFound(err, fmt.Sprintf("find user %s", id))
	}
	return u, nil
}
