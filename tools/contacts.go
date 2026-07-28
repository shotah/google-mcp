package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	people "google.golang.org/api/people/v1"

	"github.com/shotah/google-workspace-mcp-go/internal/google"
	"github.com/shotah/google-workspace-mcp-go/server"
)

const (
	defaultPersonFields  = "names,emailAddresses,phoneNumbers,organizations"
	detailedPersonFields = "names,emailAddresses,phoneNumbers,organizations,biographies,addresses,birthdays,urls,photos,metadata,memberships"
	contactGroupFields   = "name,groupType,memberCount,metadata"
)

// searchCacheWarmed tracks whether the People API search cache has been
// warmed up for each user email.
var (
	searchCacheWarmed   = make(map[string]bool)
	searchCacheWarmedMu sync.Mutex
)

// RegisterContactsTools registers all Contacts tools with the MCP server.
func RegisterContactsTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	// Read tools (US-021)
	registerListContacts(s, getClient)
	registerGetContact(s, getClient)
	registerSearchContacts(s, getClient)
	registerListContactGroups(s, getClient)
	registerGetContactGroup(s, getClient)

	// Write tools (US-022)
	registerCreateContact(s, getClient)
	registerUpdateContact(s, getClient)
	registerDeleteContact(s, getClient)
	registerBatchCreateContacts(s, getClient)
	registerBatchUpdateContacts(s, getClient)
	registerBatchDeleteContacts(s, getClient)
	registerCreateContactGroup(s, getClient)
	registerUpdateContactGroup(s, getClient)
	registerDeleteContactGroup(s, getClient)
	registerModifyContactGroupMembers(s, getClient)
}

// newPeopleService creates a people.Service for the given user email.
func newPeopleService(ctx context.Context, getClient httpClientFunc, email string) (*people.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := people.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating People service: %w", err)
	}
	return svc, nil
}

// warmupSearchCache warms up the People API search cache for the given user.
// The People API requires an initial empty query before searches return results.
func warmupSearchCache(svc *people.Service, email string) {
	searchCacheWarmedMu.Lock()
	if searchCacheWarmed[email] {
		searchCacheWarmedMu.Unlock()
		return
	}
	searchCacheWarmedMu.Unlock()

	// Fire and ignore errors — warmup failure is non-fatal.
	_, _ = svc.People.SearchContacts().Query("").ReadMask("names").PageSize(1).Do()

	searchCacheWarmedMu.Lock()
	searchCacheWarmed[email] = true
	searchCacheWarmedMu.Unlock()
}

// formatContact formats a Person resource into a readable string.
func formatContact(person *people.Person, detailed bool) string {
	if person == nil {
		return ""
	}

	resourceName := person.ResourceName
	contactID := strings.TrimPrefix(resourceName, "people/")
	if contactID == "" {
		contactID = "Unknown"
	}

	var lines []string
	lines = append(lines, "Contact ID: "+contactID)

	// Names
	if len(person.Names) > 0 {
		displayName := person.Names[0].DisplayName
		if displayName != "" {
			lines = append(lines, "Name: "+displayName)
		}
	}

	// Email addresses
	if len(person.EmailAddresses) > 0 {
		var emails []string
		for _, e := range person.EmailAddresses {
			if e.Value != "" {
				emails = append(emails, e.Value)
			}
		}
		if len(emails) > 0 {
			lines = append(lines, "Email: "+strings.Join(emails, ", "))
		}
	}

	// Phone numbers
	if len(person.PhoneNumbers) > 0 {
		var phones []string
		for _, p := range person.PhoneNumbers {
			if p.Value != "" {
				phones = append(phones, p.Value)
			}
		}
		if len(phones) > 0 {
			lines = append(lines, "Phone: "+strings.Join(phones, ", "))
		}
	}

	// Organizations
	if len(person.Organizations) > 0 {
		org := person.Organizations[0]
		var orgParts []string
		if org.Title != "" {
			orgParts = append(orgParts, org.Title)
		}
		if org.Name != "" {
			orgParts = append(orgParts, "at "+org.Name)
		}
		if len(orgParts) > 0 {
			lines = append(lines, "Organization: "+strings.Join(orgParts, " "))
		}
	}

	if detailed {
		lines = append(lines, formatDetailedContact(person)...)
	}

	return strings.Join(lines, "\n")
}

func formatDetailedContact(person *people.Person) []string {
	var lines []string
	if len(person.Addresses) > 0 && person.Addresses[0].FormattedValue != "" {
		lines = append(lines, "Address: "+person.Addresses[0].FormattedValue)
	}
	if len(person.Birthdays) > 0 && person.Birthdays[0].Date != nil {
		bday := person.Birthdays[0].Date
		bdayStr := fmt.Sprintf("%d/%d", bday.Month, bday.Day)
		if bday.Year > 0 {
			bdayStr = fmt.Sprintf("%d/%s", bday.Year, bdayStr)
		}
		lines = append(lines, "Birthday: "+bdayStr)
	}
	var urls []string
	for _, u := range person.Urls {
		if u.Value != "" {
			urls = append(urls, u.Value)
		}
	}
	if len(urls) > 0 {
		lines = append(lines, "URLs: "+strings.Join(urls, ", "))
	}
	if len(person.Biographies) > 0 {
		bio := person.Biographies[0].Value
		if len(bio) > 200 {
			bio = bio[:200] + "..."
		}
		if bio != "" {
			lines = append(lines, "Notes: "+bio)
		}
	}
	var sourceTypes []string
	if person.Metadata != nil {
		for _, src := range person.Metadata.Sources {
			if src.Type != "" {
				sourceTypes = append(sourceTypes, src.Type)
			}
		}
	}
	if len(sourceTypes) > 0 {
		lines = append(lines, "Sources: "+strings.Join(sourceTypes, ", "))
	}
	return lines
}

// ---------------------------------------------------------------------------
// list_contacts
// ---------------------------------------------------------------------------

func registerListContacts(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("list_contacts",
		mcp.WithDescription("List contacts (names, emails, phones). Use for 'show my contacts', browse address book. Not targeted lookup — use search_contacts or get_contact."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithNumber("page_size", mcp.Description("Maximum number of contacts to return (default: 100, max: 1000).")),
		mcp.WithString("page_token", mcp.Description("Token for pagination.")),
		mcp.WithString("sort_order", mcp.Description("Sort order."),
			mcp.Enum("LAST_MODIFIED_ASCENDING", "LAST_MODIFIED_DESCENDING", "FIRST_NAME_ASCENDING", "LAST_NAME_ASCENDING")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pageSize := min(request.GetInt("page_size", 100), 1000)
		pageToken := request.GetString("page_token", "")
		sortOrder := request.GetString("sort_order", "")

		call := svc.People.Connections.List("people/me").
			PersonFields(defaultPersonFields).
			PageSize(int64(pageSize))

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		if sortOrder != "" {
			call = call.SortOrder(sortOrder)
		}

		result, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing contacts: %v", err)), nil
		}

		connections := result.Connections
		if len(connections) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No contacts found for %s.", email)), nil
		}

		totalPeople := int(result.TotalPeople)
		if totalPeople == 0 {
			totalPeople = len(connections)
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contacts for %s (%d of %d):\n\n", email, len(connections), totalPeople)

		for _, person := range connections {
			sb.WriteString(formatContact(person, false))
			sb.WriteString("\n\n")
		}

		if result.NextPageToken != "" {
			fmt.Fprintf(&sb, "Next page token: %s", result.NextPageToken)
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// get_contact
// ---------------------------------------------------------------------------

func registerGetContact(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("get_contact",
		mcp.WithDescription("Get one contact by resource_name/id (full fields). Use after search_contacts. Not for groups — use get_contact_group."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("contact_id", mcp.Required(), mcp.Description("The contact ID (e.g., \"c1234567890\" or full resource name \"people/c1234567890\").")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		contactID, err := request.RequireString("contact_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Normalize resource name
		resourceName := contactID
		if !strings.HasPrefix(contactID, "people/") {
			resourceName = "people/" + contactID
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		person, err := svc.People.Get(resourceName).PersonFields(detailedPersonFields).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting contact %s: %v", contactID, err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contact Details for %s:\n\n", email)
		sb.WriteString(formatContact(person, true))

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// search_contacts
// ---------------------------------------------------------------------------

func registerSearchContacts(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("search_contacts",
		mcp.WithDescription("Search contacts by name, email, phone, etc. Use for 'find John's email'. Prefer get_contact when you already have the contact id."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query string (searches names, emails, phone numbers).")),
		mcp.WithNumber("page_size", mcp.Description("Maximum number of results to return (default: 30, max: 30).")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Warm up the search cache if needed
		warmupSearchCache(svc, email)

		pageSize := min(request.GetInt("page_size", 30), 30)

		result, err := svc.People.SearchContacts().
			Query(query).
			ReadMask(defaultPersonFields).
			PageSize(int64(pageSize)).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("searching contacts: %v", err)), nil
		}

		results := result.Results
		if len(results) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No contacts found matching '%s' for %s.", query, email)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Search Results for '%s' (%d found):\n\n", query, len(results))

		for _, item := range results {
			sb.WriteString(formatContact(item.Person, false))
			sb.WriteString("\n\n")
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// list_contact_groups
// ---------------------------------------------------------------------------

func registerListContactGroups(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("list_contact_groups",
		mcp.WithDescription("List contact groups/labels (group ids + names). Use before modify_contact_group_members. Not individual people — use list_contacts."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithNumber("page_size", mcp.Description("Maximum number of groups to return (default: 100, max: 1000).")),
		mcp.WithString("page_token", mcp.Description("Token for pagination.")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pageSize := min(request.GetInt("page_size", 100), 1000)
		pageToken := request.GetString("page_token", "")

		call := svc.ContactGroups.List().
			PageSize(int64(pageSize)).
			GroupFields(contactGroupFields)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing contact groups: %v", err)), nil
		}

		groups := result.ContactGroups
		if len(groups) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No contact groups found for %s.", email)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contact Groups for %s:\n\n", email)

		for _, group := range groups {
			groupID := strings.TrimPrefix(group.ResourceName, "contactGroups/")
			name := group.Name
			if name == "" {
				name = "Unnamed"
			}
			groupType := group.GroupType
			if groupType == "" {
				groupType = "USER_CONTACT_GROUP"
			}

			fmt.Fprintf(&sb, "- %s\n", name)
			fmt.Fprintf(&sb, "  ID: %s\n", groupID)
			fmt.Fprintf(&sb, "  Type: %s\n", groupType)
			fmt.Fprintf(&sb, "  Members: %d\n\n", group.MemberCount)
		}

		if result.NextPageToken != "" {
			fmt.Fprintf(&sb, "Next page token: %s", result.NextPageToken)
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// normalizeContactResourceName ensures a contact ID is prefixed with "people/".
func normalizeContactResourceName(contactID string) string {
	if strings.HasPrefix(contactID, "people/") {
		return contactID
	}
	return "people/" + contactID
}

// normalizeGroupResourceName ensures a group ID is prefixed with "contactGroups/".
func normalizeGroupResourceName(groupID string) string {
	if strings.HasPrefix(groupID, "contactGroups/") {
		return groupID
	}
	return "contactGroups/" + groupID
}

// buildPersonBody constructs a people.Person from optional contact fields.
// Returns nil if no fields are set.
func buildPersonBody(args map[string]any) *people.Person {
	givenName, _ := args["given_name"].(string)
	familyName, _ := args["family_name"].(string)
	email, _ := args["email"].(string)
	phone, _ := args["phone"].(string)
	organization, _ := args["organization"].(string)
	jobTitle, _ := args["job_title"].(string)
	notes, _ := args["notes"].(string)

	p := &people.Person{}
	hasField := false

	if givenName != "" || familyName != "" {
		p.Names = []*people.Name{{GivenName: givenName, FamilyName: familyName}}
		hasField = true
	}
	if email != "" {
		p.EmailAddresses = []*people.EmailAddress{{Value: email}}
		hasField = true
	}
	if phone != "" {
		p.PhoneNumbers = []*people.PhoneNumber{{Value: phone}}
		hasField = true
	}
	if organization != "" || jobTitle != "" {
		org := &people.Organization{}
		if organization != "" {
			org.Name = organization
		}
		if jobTitle != "" {
			org.Title = jobTitle
		}
		p.Organizations = []*people.Organization{org}
		hasField = true
	}
	if notes != "" {
		p.Biographies = []*people.Biography{{Value: notes, ContentType: "TEXT_PLAIN"}}
		hasField = true
	}

	if !hasField {
		return nil
	}
	return p
}

// updatePersonFields returns the comma-separated list of fields present in a Person.
func updatePersonFields(p *people.Person) string {
	var fields []string
	if len(p.Names) > 0 {
		fields = append(fields, "names")
	}
	if len(p.EmailAddresses) > 0 {
		fields = append(fields, "emailAddresses")
	}
	if len(p.PhoneNumbers) > 0 {
		fields = append(fields, "phoneNumbers")
	}
	if len(p.Organizations) > 0 {
		fields = append(fields, "organizations")
	}
	if len(p.Biographies) > 0 {
		fields = append(fields, "biographies")
	}
	if len(p.Addresses) > 0 {
		fields = append(fields, "addresses")
	}
	return strings.Join(fields, ",")
}

// ---------------------------------------------------------------------------
// get_contact_group
// ---------------------------------------------------------------------------

func registerGetContactGroup(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("get_contact_group",
		mcp.WithDescription("Get one contact group + member list by group resource name. Use before adding/removing members."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("The contact group ID.")),
		mcp.WithNumber("max_members", mcp.Description("Maximum number of members to return (default: 100, max: 1000).")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		groupID, err := request.RequireString("group_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Normalize resource name
		resourceName := groupID
		if !strings.HasPrefix(groupID, "contactGroups/") {
			resourceName = "contactGroups/" + groupID
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxMembers := min(request.GetInt("max_members", 100), 1000)

		result, err := svc.ContactGroups.Get(resourceName).
			MaxMembers(int64(maxMembers)).
			GroupFields(contactGroupFields).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting contact group %s: %v", groupID, err)), nil
		}

		name := result.Name
		if name == "" {
			name = "Unnamed"
		}
		groupType := result.GroupType
		if groupType == "" {
			groupType = "USER_CONTACT_GROUP"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contact Group Details for %s:\n\n", email)
		fmt.Fprintf(&sb, "Name: %s\n", name)
		fmt.Fprintf(&sb, "ID: %s\n", groupID)
		fmt.Fprintf(&sb, "Type: %s\n", groupType)
		fmt.Fprintf(&sb, "Total Members: %d\n", result.MemberCount)

		if len(result.MemberResourceNames) > 0 {
			fmt.Fprintf(&sb, "\nMembers (%d shown):\n", len(result.MemberResourceNames))
			for _, member := range result.MemberResourceNames {
				contactID := strings.TrimPrefix(member, "people/")
				fmt.Fprintf(&sb, "  - %s\n", contactID)
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// create_contact
// ---------------------------------------------------------------------------

func registerCreateContact(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("create_contact",
		mcp.WithDescription("Create a new contact (name, emails, phones, etc.). Use for 'add someone to contacts'. Prefer batch_create_contacts for many."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("given_name", mcp.Description("First name.")),
		mcp.WithString("family_name", mcp.Description("Last name.")),
		mcp.WithString("email", mcp.Description("Email address.")),
		mcp.WithString("phone", mcp.Description("Phone number.")),
		mcp.WithString("organization", mcp.Description("Company/organization name.")),
		mcp.WithString("job_title", mcp.Description("Job title.")),
		mcp.WithString("notes", mcp.Description("Additional notes.")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		person := buildPersonBody(request.GetArguments())
		if person == nil {
			return mcp.NewToolResultError("At least one field (name, email, phone, etc.) must be provided."), nil
		}

		result, err := svc.People.CreateContact(person).
			PersonFields(detailedPersonFields).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating contact: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contact Created for %s:\n\n", email)
		sb.WriteString(formatContact(result, true))

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// update_contact
// ---------------------------------------------------------------------------

func registerUpdateContact(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("update_contact",
		mcp.WithDescription("Replace fields on an existing contact (full replace, not merge). Required contact resource name. Confirm before overwriting data."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("contact_id", mcp.Required(), mcp.Description("The contact ID to update.")),
		mcp.WithString("given_name", mcp.Description("New first name.")),
		mcp.WithString("family_name", mcp.Description("New last name.")),
		mcp.WithString("email", mcp.Description("New email address.")),
		mcp.WithString("phone", mcp.Description("New phone number.")),
		mcp.WithString("organization", mcp.Description("New company/organization name.")),
		mcp.WithString("job_title", mcp.Description("New job title.")),
		mcp.WithString("notes", mcp.Description("New notes.")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		contactID, err := request.RequireString("contact_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resourceName := normalizeContactResourceName(contactID)

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Fetch current contact for etag
		current, err := svc.People.Get(resourceName).PersonFields(detailedPersonFields).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting contact %s: %v", contactID, err)), nil
		}
		if current.Etag == "" {
			return mcp.NewToolResultError("Unable to get contact etag for update."), nil
		}

		person := buildPersonBody(request.GetArguments())
		if person == nil {
			return mcp.NewToolResultError("At least one field (name, email, phone, etc.) must be provided."), nil
		}
		person.Etag = current.Etag

		upf := updatePersonFields(person)
		if upf == "" {
			return mcp.NewToolResultError("No fields to update."), nil
		}

		result, err := svc.People.UpdateContact(resourceName, person).
			UpdatePersonFields(upf).
			PersonFields(detailedPersonFields).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("updating contact %s: %v", contactID, err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contact Updated for %s:\n\n", email)
		sb.WriteString(formatContact(result, true))

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// delete_contact
// ---------------------------------------------------------------------------

func registerDeleteContact(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("delete_contact",
		mcp.WithDescription("Delete one contact by resource name. Destructive — confirm when unclear. Prefer batch_delete_contacts for many."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("contact_id", mcp.Required(), mcp.Description("The contact ID to delete.")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		contactID, err := request.RequireString("contact_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resourceName := normalizeContactResourceName(contactID)

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		_, err = svc.People.DeleteContact(resourceName).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("deleting contact %s: %v", contactID, err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Contact %s has been deleted for %s.", contactID, email)), nil
	})
}

// ---------------------------------------------------------------------------
// batch_create_contacts
// ---------------------------------------------------------------------------

func registerBatchCreateContacts(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("batch_create_contacts",
		mcp.WithDescription("Create many contacts in one call. Use for imports/bulk add. Prefer create_contact for one person."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithArray("contacts", mcp.Required(), mcp.Description("List of contact objects with fields: given_name, family_name, email, phone, organization, job_title."),
			mcp.Items(map[string]any{"type": "object"})),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		args := request.GetArguments()
		contactsRaw, ok := args["contacts"].([]any)
		if !ok || len(contactsRaw) == 0 {
			return mcp.NewToolResultError("At least one contact must be provided."), nil
		}
		if len(contactsRaw) > 200 {
			return mcp.NewToolResultError("Maximum 200 contacts can be created in a batch."), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var contacts []*people.ContactToCreate
		for _, raw := range contactsRaw {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			person := buildPersonBody(m)
			if person != nil {
				contacts = append(contacts, &people.ContactToCreate{ContactPerson: person})
			}
		}

		if len(contacts) == 0 {
			return mcp.NewToolResultError("No valid contact data provided."), nil
		}

		result, err := svc.People.BatchCreateContacts(&people.BatchCreateContactsRequest{
			Contacts: contacts,
			ReadMask: defaultPersonFields,
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("batch creating contacts: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Batch Create Results for %s:\n\n", email)
		fmt.Fprintf(&sb, "Created %d contacts:\n\n", len(result.CreatedPeople))

		for _, item := range result.CreatedPeople {
			if item.Person != nil {
				sb.WriteString(formatContact(item.Person, false))
				sb.WriteString("\n\n")
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// batch_update_contacts
// ---------------------------------------------------------------------------

func registerBatchUpdateContacts(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("batch_update_contacts",
		mcp.WithDescription("Update many contacts (replace fields each). Confirm bulk overwrites. Prefer update_contact for one."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithArray("updates", mcp.Required(), mcp.Description("List of update objects with fields: contact_id (required), given_name, family_name, email, phone, organization, job_title."),
			mcp.Items(map[string]any{"type": "object"})),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		args := request.GetArguments()
		updatesRaw, ok := args["updates"].([]any)
		if !ok || len(updatesRaw) == 0 {
			return mcp.NewToolResultError("At least one update must be provided."), nil
		}
		if len(updatesRaw) > 200 {
			return mcp.NewToolResultError("Maximum 200 contacts can be updated in a batch."), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Collect resource names for batch etag lookup
		var resourceNames []string
		for _, raw := range updatesRaw {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			cid, _ := m["contact_id"].(string)
			if cid == "" {
				return mcp.NewToolResultError("Each update must include a contact_id."), nil
			}
			resourceNames = append(resourceNames, normalizeContactResourceName(cid))
		}

		// Batch fetch etags
		batchGet, err := svc.People.GetBatchGet().
			ResourceNames(resourceNames...).
			PersonFields("metadata").
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetching contact etags: %v", err)), nil
		}

		etags := make(map[string]string)
		for _, resp := range batchGet.Responses {
			if resp.Person != nil && resp.Person.ResourceName != "" && resp.Person.Etag != "" {
				etags[resp.Person.ResourceName] = resp.Person.Etag
			}
		}

		// Build batch update body
		contactsMap := make(map[string]people.Person)
		updateFieldsSet := make(map[string]bool)

		for _, raw := range updatesRaw {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			cid, _ := m["contact_id"].(string)
			rn := normalizeContactResourceName(cid)

			etag, ok := etags[rn]
			if !ok {
				continue
			}

			person := buildPersonBody(m)
			if person == nil {
				continue
			}
			person.ResourceName = rn
			person.Etag = etag

			// Track update fields
			for f := range strings.SplitSeq(updatePersonFields(person), ",") {
				if f != "" {
					updateFieldsSet[f] = true
				}
			}

			contactsMap[rn] = *person
		}

		if len(contactsMap) == 0 {
			return mcp.NewToolResultError("No valid update data provided."), nil
		}

		var updateMaskParts []string
		for f := range updateFieldsSet {
			updateMaskParts = append(updateMaskParts, f)
		}

		result, err := svc.People.BatchUpdateContacts(&people.BatchUpdateContactsRequest{
			Contacts:   contactsMap,
			UpdateMask: strings.Join(updateMaskParts, ","),
			ReadMask:   defaultPersonFields,
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("batch updating contacts: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Batch Update Results for %s:\n\n", email)
		fmt.Fprintf(&sb, "Updated %d contacts:\n\n", len(result.UpdateResult))

		for _, personResp := range result.UpdateResult {
			if personResp.Person != nil {
				sb.WriteString(formatContact(personResp.Person, false))
				sb.WriteString("\n\n")
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// batch_delete_contacts
// ---------------------------------------------------------------------------

func registerBatchDeleteContacts(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("batch_delete_contacts",
		mcp.WithDescription("Delete many contacts by resource names. Destructive — confirm when unclear."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithArray("contact_ids", mcp.Required(), mcp.Description("List of contact IDs to delete."),
			mcp.Items(map[string]any{"type": "string"})),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		contactIDs, err := request.RequireStringSlice("contact_ids")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(contactIDs) == 0 {
			return mcp.NewToolResultError("At least one contact ID must be provided."), nil
		}
		if len(contactIDs) > 500 {
			return mcp.NewToolResultError("Maximum 500 contacts can be deleted in a batch."), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resourceNames := make([]string, len(contactIDs))
		for i, cid := range contactIDs {
			resourceNames[i] = normalizeContactResourceName(cid)
		}

		_, err = svc.People.BatchDeleteContacts(&people.BatchDeleteContactsRequest{
			ResourceNames: resourceNames,
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("batch deleting contacts: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Batch deleted %d contacts for %s.", len(contactIDs), email)), nil
	})
}

// ---------------------------------------------------------------------------
// create_contact_group
// ---------------------------------------------------------------------------

func registerCreateContactGroup(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("create_contact_group",
		mcp.WithDescription("Create a contact group/label. Use for 'new group called…'. Add members with modify_contact_group_members."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("name", mcp.Required(), mcp.Description("The name of the new contact group.")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		name, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := svc.ContactGroups.Create(&people.CreateContactGroupRequest{
			ContactGroup: &people.ContactGroup{Name: name},
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating contact group: %v", err)), nil
		}

		groupID := strings.TrimPrefix(result.ResourceName, "contactGroups/")
		createdName := result.Name
		if createdName == "" {
			createdName = name
		}
		groupType := result.GroupType
		if groupType == "" {
			groupType = "USER_CONTACT_GROUP"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contact Group Created for %s:\n\n", email)
		fmt.Fprintf(&sb, "Name: %s\n", createdName)
		fmt.Fprintf(&sb, "ID: %s\n", groupID)
		fmt.Fprintf(&sb, "Type: %s\n", groupType)

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// update_contact_group
// ---------------------------------------------------------------------------

func registerUpdateContactGroup(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("update_contact_group",
		mcp.WithDescription("Rename a contact group. Required group resource name. Not for membership — use modify_contact_group_members."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("The contact group ID to update.")),
		mcp.WithString("name", mcp.Required(), mcp.Description("The new name for the contact group.")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		groupID, err := request.RequireString("group_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		name, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resourceName := normalizeGroupResourceName(groupID)

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := svc.ContactGroups.Update(resourceName, &people.UpdateContactGroupRequest{
			ContactGroup: &people.ContactGroup{Name: name},
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("updating contact group %s: %v", groupID, err)), nil
		}

		updatedName := result.Name
		if updatedName == "" {
			updatedName = name
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contact Group Updated for %s:\n\n", email)
		fmt.Fprintf(&sb, "Name: %s\n", updatedName)
		fmt.Fprintf(&sb, "ID: %s\n", groupID)

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// ---------------------------------------------------------------------------
// delete_contact_group
// ---------------------------------------------------------------------------

func registerDeleteContactGroup(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("delete_contact_group",
		mcp.WithDescription("Delete a contact group (label). Does not delete contacts. Destructive — confirm when unclear."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("The contact group ID to delete.")),
		mcp.WithBoolean("delete_contacts", mcp.Description("If true, also delete contacts in the group (default: false).")),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		groupID, err := request.RequireString("group_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resourceName := normalizeGroupResourceName(groupID)
		deleteContacts := getBool(request, "delete_contacts", false)

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		_, err = svc.ContactGroups.Delete(resourceName).
			DeleteContacts(deleteContacts).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("deleting contact group %s: %v", groupID, err)), nil
		}

		msg := fmt.Sprintf("Contact group %s has been deleted for %s.", groupID, email)
		if deleteContacts {
			msg += " Contacts in the group were also deleted."
		} else {
			msg += " Contacts in the group were preserved."
		}

		return mcp.NewToolResultText(msg), nil
	})
}

// ---------------------------------------------------------------------------
// modify_contact_group_members
// ---------------------------------------------------------------------------

func registerModifyContactGroupMembers(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("modify_contact_group_members",
		mcp.WithDescription("Add or remove contacts from a group. Required group id + contact resource names. Use after list_contact_groups / search_contacts."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("The contact group ID.")),
		mcp.WithArray("add_contact_ids", mcp.Description("Contact IDs to add to the group."),
			mcp.Items(map[string]any{"type": "string"})),
		mcp.WithArray("remove_contact_ids", mcp.Description("Contact IDs to remove from the group."),
			mcp.Items(map[string]any{"type": "string"})),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		groupID, err := request.RequireString("group_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resourceName := normalizeGroupResourceName(groupID)

		addIDs := getStringSlice(request, "add_contact_ids")
		removeIDs := getStringSlice(request, "remove_contact_ids")

		if len(addIDs) == 0 && len(removeIDs) == 0 {
			return mcp.NewToolResultError("At least one of add_contact_ids or remove_contact_ids must be provided."), nil
		}

		svc, err := newPeopleService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body := &people.ModifyContactGroupMembersRequest{}

		if len(addIDs) > 0 {
			addNames := make([]string, len(addIDs))
			for i, cid := range addIDs {
				addNames[i] = normalizeContactResourceName(cid)
			}
			body.ResourceNamesToAdd = addNames
		}
		if len(removeIDs) > 0 {
			removeNames := make([]string, len(removeIDs))
			for i, cid := range removeIDs {
				removeNames[i] = normalizeContactResourceName(cid)
			}
			body.ResourceNamesToRemove = removeNames
		}

		result, err := svc.ContactGroups.Members.Modify(resourceName, body).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("modifying contact group members %s: %v", groupID, err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Contact Group Members Modified for %s:\n\n", email)
		fmt.Fprintf(&sb, "Group: %s\n", groupID)

		if len(addIDs) > 0 {
			fmt.Fprintf(&sb, "Added: %d contacts\n", len(addIDs))
		}
		if len(removeIDs) > 0 {
			fmt.Fprintf(&sb, "Removed: %d contacts\n", len(removeIDs))
		}

		if len(result.NotFoundResourceNames) > 0 {
			fmt.Fprintf(&sb, "\nNot found: %s\n", strings.Join(result.NotFoundResourceNames, ", "))
		}
		if len(result.CanNotRemoveLastContactGroupResourceNames) > 0 {
			fmt.Fprintf(&sb, "\nCannot remove (last group): %s\n", strings.Join(result.CanNotRemoveLastContactGroupResourceNames, ", "))
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}
