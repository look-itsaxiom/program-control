package store

import (
	"context"
)

// ProgramAccess is what a caller is allowed to do with one program: which organization owns it,
// and which of the caller's own organizations grants them the access.
type ProgramAccess struct {
	ProgramID string
	Name      string
	OwnerOrg  string

	// CallerOrg is the caller's organization that reaches this program: the owning organization
	// when they are a member of it, otherwise the shared organization they came in through. It is
	// the organization a task they create belongs to.
	CallerOrg string
}

// IsOwner reports whether the caller reached the program through the organization that owns it.
func (a ProgramAccess) IsOwner() bool { return a.CallerOrg == a.OwnerOrg }

// ProgramAccessFor resolves how orgIDs reach programID. A caller in neither the owning
// organization nor any organization the program is shared with gets ErrNotFound, the same answer
// a program that never existed gives, so the routes above cannot leak its existence.
func (s *Store) ProgramAccessFor(ctx context.Context, programID string, orgIDs []string) (ProgramAccess, error) {
	const q = `
		select
			p.name,
			p.org_id,
			coalesce(
				(select o from unnest($2::uuid[]) as o where o = p.org_id),
				(select s.org_id from program_shares s
				 where s.program_id = p.id and s.org_id = any($2::uuid[]) limit 1)
			) as caller_org_id
		from programs p
		where p.id = $1 and p.deleted_at is null
	`
	access := ProgramAccess{ProgramID: programID}
	var callerOrg *string
	if err := s.pool.QueryRow(ctx, q, programID, orgIDs).Scan(&access.Name, &access.OwnerOrg, &callerOrg); err != nil {
		return ProgramAccess{}, notFound(err, "program access")
	}
	if callerOrg == nil {
		return ProgramAccess{}, ErrNotFound
	}
	access.CallerOrg = *callerOrg
	return access, nil
}
