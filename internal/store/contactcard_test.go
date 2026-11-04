package store

import (
	"context"
	"testing"

	"github.com/shukiv/whatsappgo/internal/model"
)

// The list people pick from when sharing a contact is sorted by the name held
// for each one, and someone who is not in the address book has no name:
// WhatsApp hands out a masked "+972∙∙∙∙∙∙∙10", a bare string of digits, or a
// single dot. Those sort before any letter, so with a hundred of them the
// reader scrolled a list of numbers and never reached anybody they knew.
func TestShareableContactsPutsThePeopleWithNamesFirst(t *testing.T) {
	st, err := Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	for _, chat := range []model.Chat{
		{JID: "972500000001@s.whatsapp.net", Title: "+972∙∙∙∙∙∙∙01"},
		{JID: "972500000002@s.whatsapp.net", Title: "."},
		{JID: "972500000003@s.whatsapp.net", Title: "972500000003"},
		{JID: "972500000004@s.whatsapp.net", Title: "+972 50-000 00.04"},
		{JID: "972500000005@s.whatsapp.net", Title: "שוקי וקנין"},
		{JID: "972500000006@s.whatsapp.net", Title: "Alice Example"},
	} {
		if err := st.UpsertChat(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}

	found, err := st.ShareableContacts(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 6 {
		t.Fatalf("contacts = %d, wanted 6", len(found))
	}
	// Everyone with a name comes first, in name order, whatever script the
	// name is written in.
	if found[0].Name != "Alice Example" || found[1].Name != "שוקי וקנין" {
		t.Fatalf("named contacts did not come first: %q, %q", found[0].Name, found[1].Name)
	}
	// And the rest are shown by number. A mask hides digits the window
	// already knows, and a dot says nothing at all.
	for _, contact := range found[2:] {
		if contact.Name != contact.Phone {
			t.Fatalf("a contact with no name was listed as %q rather than %q", contact.Name, contact.Phone)
		}
	}
}

// Searching still reaches somebody by name or by number, wherever they fall in
// that order.
func TestShareableContactsAreStillSearchable(t *testing.T) {
	st, err := Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	for _, chat := range []model.Chat{
		{JID: "972500000007@s.whatsapp.net", Title: "Bob Example"},
		{JID: "972500000008@s.whatsapp.net", Title: "+972∙∙∙∙∙∙∙08"},
	} {
		if err := st.UpsertChat(ctx, chat); err != nil {
			t.Fatal(err)
		}
	}
	byName, err := st.ShareableContacts(ctx, "Bob")
	if err != nil || len(byName) != 1 || byName[0].Name != "Bob Example" {
		t.Fatalf("searching by name found %#v (%v)", byName, err)
	}
	byNumber, err := st.ShareableContacts(ctx, "972500000008")
	if err != nil || len(byNumber) != 1 || byNumber[0].Phone != "+972500000008" {
		t.Fatalf("searching by number found %#v (%v)", byNumber, err)
	}
}
