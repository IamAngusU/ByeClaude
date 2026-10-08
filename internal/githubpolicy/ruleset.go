package githubpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/IamAngusU/ByeClaude/internal/attribution"
)

// Repository rulesets are a server-side rejection guard. GitHub does not
// rewrite commit messages: matching pushes are denied. The regex necessarily
// approximates ByeClaude's final-trailer parser; document this distinction.
type PatternParameters struct {
	Name string `json:"name,omitempty"`
	Negate bool `json:"negate"`
	Operator string `json:"operator"`
	Pattern string `json:"pattern"`
}
type Rule struct {
	Type string `json:"type"`
	Parameters PatternParameters `json:"parameters"`
}
type Ruleset struct {
	ID int64 `json:"id,omitempty"`
	Name string `json:"name"`
	Target string `json:"target"`
	Enforcement string `json:"enforcement"`
	Conditions struct{
		RefName struct{
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"ref_name"`
	} `json:"conditions"`
	Rules []Rule `json:"rules"`
}
var ownerRepoRE = regexp.MustCompile("^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})/[A-Za-z0-9_.-]{1,100}$")
func ValidateRepo(slug string) error {
	if !ownerRepoRE.MatchString(slug) || strings.Contains(slug,"..") {
		return fmt.Errorf("repository must be a canonical GitHub OWNER/NAME")
	}
	return nil
}

func GenerateRuleset(matcher attribution.Matcher, includeIdentityEmails bool) (Ruleset, error) {
	var rules []attribution.Rule
	switch v:=matcher.(type) {
	case attribution.Rule: rules=[]attribution.Rule{v}
	case attribution.RuleSet: rules=v.Rules
	default: return Ruleset{},fmt.Errorf("ruleset export requires a structured ByeClaude rule or ruleset")
	}
	if len(rules)==0 {return Ruleset{},fmt.Errorf("no rules to export")}
	var messagePatterns, emailPatterns []string
	for _, r:=range rules {
		message,email,err:=patternsForRule(r)
		if err!=nil {return Ruleset{},fmt.Errorf("rule %q: %w",r.ID(),err)}
		messagePatterns=append(messagePatterns,message)
		if email!="" {emailPatterns=append(emailPatterns,email)}
	}
	out:=Ruleset{Name:"ByeClaude attribution guard",Target:"branch",Enforcement:"active"}
	out.Conditions.RefName.Include=[]string{"~ALL"}
	out.Conditions.RefName.Exclude=[]string{}
	out.Rules=[]Rule{{
		Type:"commit_message_pattern",
		Parameters:PatternParameters{Name:"Reject matching Co-Authored-By trailers",Negate:true,Operator:"regex",Pattern:"(?im)^(?:"+strings.Join(messagePatterns,"|")+")$"},
	}}
	if includeIdentityEmails && len(emailPatterns)>0 {
		re:= "(?i)^(?:"+strings.Join(emailPatterns,"|")+")$"
		out.Rules=append(out.Rules,Rule{Type:"commit_author_email_pattern",Parameters:PatternParameters{Name:"Reject matching Git author emails",Negate:true,Operator:"regex",Pattern:re}})
		out.Rules=append(out.Rules,Rule{Type:"committer_email_pattern",Parameters:PatternParameters{Name:"Reject matching Git committer emails",Negate:true,Operator:"regex",Pattern:re}})
	}
	for _, rule:=range out.Rules {
		if _,err:=regexp.Compile(rule.Parameters.Pattern);err!=nil {return Ruleset{},fmt.Errorf("invalid generated regex for %s: %w",rule.Type,err)}
	}
	return out,nil
}

func patternsForRule(r attribution.Rule) (message,email string,err error) {
	var names,emails []string
	for _, n:=range r.NameContains {
		n=strings.TrimSpace(n)
		if n!="" {names=append(names,regexp.QuoteMeta(n))}
	}
	for _, exact:=range r.ExactEmails {
		exact=strings.TrimSpace(exact)
		if exact!="" {emails=append(emails,regexp.QuoteMeta(exact))}
	}
	for _, domain:=range r.EmailDomains {
		domain=strings.TrimSpace(strings.TrimPrefix(domain,"@"))
		if domain!="" {emails=append(emails,"[^<>@ \t\r\n]+@"+regexp.QuoteMeta(domain))}
	}
	if len(names)==0 && len(emails)==0 {
		return "","",fmt.Errorf("empty identity rule cannot be exported")
	}
	namePattern:="[^<\r\n]*"
	if len(names)>0 {
		namePattern="[^<\r\n]*(?:"+strings.Join(names,"|")+")[^<\r\n]*"
	}
	emailPattern:="[^<> \t\r\n]+"
	if len(emails)>0 {
		emailPattern="(?:"+strings.Join(emails,"|")+")"
	}
	return "[ \t]*co-authored-by:[ \t]*"+namePattern+"<"+emailPattern+">[ \t]*",strings.Join(emails,"|"),nil
}

type Client struct {
	Token string
	BaseURL string
	HTTPClient *http.Client
}
func (c Client) api() string {
	if c.BaseURL!="" {return strings.TrimRight(c.BaseURL,"/")}
	return "https://api.github.com"
}
func (c Client) client() *http.Client {
	if c.HTTPClient!=nil {return c.HTTPClient}
	return http.DefaultClient
}
func (c Client) do(ctx context.Context, method, path string, body any, destination any) error {
	var data []byte
	if body!=nil {
		var err error
		data,err=json.Marshal(body)
		if err!=nil{return err}
	}
	req,err:=http.NewRequestWithContext(ctx,method,c.api()+path,bytes.NewReader(data))
	if err!=nil{return err}
	req.Header.Set("Accept","application/vnd.github+json")
	req.Header.Set("User-Agent","ByeClaude")
	req.Header.Set("X-GitHub-Api-Version","2022-11-28")
	if body!=nil{req.Header.Set("Content-Type","application/json")}
	if c.Token!="" {req.Header.Set("Authorization","Bearer "+c.Token)}
	resp,err:=c.client().Do(req)
	if err!=nil{return fmt.Errorf("GitHub API request failed: %w",err)}
	defer resp.Body.Close()
	if resp.StatusCode<200 || resp.StatusCode>=300 {
		return fmt.Errorf("GitHub %s %s returned HTTP %d",method,path,resp.StatusCode)
	}
	if destination!=nil {
		return json.NewDecoder(resp.Body).Decode(destination)
	}
	return nil
}
func (c Client) ListRulesets(ctx context.Context, repo string) ([]Ruleset,error) {
	if err:=ValidateRepo(repo);err!=nil{return nil,err}
	var all []Ruleset
	for page:=1;page<=20;page++ {
		var batch []Ruleset
		path:="/repos/"+repo+"/rulesets?per_page=100&includes_parents=false&page="+fmt.Sprint(page)
		if err:=c.do(ctx,http.MethodGet,path,nil,&batch);err!=nil{return nil,err}
		all=append(all,batch...)
		if len(batch)<100{return all,nil}
	}
	return nil,fmt.Errorf("ruleset listing exceeded 2,000 entries; refusing incomplete duplicate check")
}
func (c Client) InstallRuleset(ctx context.Context,repo string,policy Ruleset) (Ruleset,error) {
	if err:=ValidateRepo(repo);err!=nil{return Ruleset{},err}
	if strings.TrimSpace(c.Token)=="" {return Ruleset{},fmt.Errorf("GH_TOKEN or GITHUB_TOKEN with repository administration permissions is required")}
	existing,err:=c.ListRulesets(ctx,repo)
	if err!=nil{return Ruleset{},err}
	for _,r:=range existing {
		if r.Name==policy.Name {return Ruleset{},fmt.Errorf("ruleset %q already exists; refusing to overwrite",policy.Name)}
	}
	var created Ruleset
	if err:=c.do(ctx,http.MethodPost,"/repos/"+repo+"/rulesets",policy,&created);err!=nil{return Ruleset{},err}
	if created.ID<=0 {return Ruleset{},fmt.Errorf("GitHub returned no ruleset ID")}
	var confirmed Ruleset
	if err:=c.do(ctx,http.MethodGet,"/repos/"+repo+"/rulesets/"+url.PathEscape(fmt.Sprint(created.ID)),nil,&confirmed);err!=nil{return created,fmt.Errorf("ruleset created but read-back failed: %w",err)}
	if confirmed.ID!=created.ID || confirmed.Enforcement!=policy.Enforcement || len(confirmed.Rules)!=len(policy.Rules) {
		return created,fmt.Errorf("ruleset created but GitHub read-back differs; inspect it manually")
	}
	return confirmed,nil
}
