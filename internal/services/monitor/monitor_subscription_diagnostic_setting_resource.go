package monitor

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/response"
	authRuleParse "github.com/hashicorp/go-azure-sdk/resource-manager/eventhub/2021-11-01/authorizationrulesnamespaces"
	"github.com/hashicorp/go-azure-sdk/resource-manager/insights/2021-05-01-preview/subscriptiondiagnosticsettings"
	"github.com/hashicorp/go-azure-sdk/resource-manager/operationalinsights/2020-08-01/workspaces"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	eventhubValidate "github.com/hashicorp/terraform-provider-azurerm/internal/services/eventhub/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/monitor/validate"
	storageParse "github.com/hashicorp/terraform-provider-azurerm/internal/services/storage/parse"
	storageValidate "github.com/hashicorp/terraform-provider-azurerm/internal/services/storage/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

func resourceMonitorSubscriptionDiagnosticSetting() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceMonitorSubscriptionDiagnosticSettingCreateUpdate,
		Read:   resourceMonitorSubscriptionDiagnosticSettingRead,
		Update: resourceMonitorSubscriptionDiagnosticSettingCreateUpdate,
		Delete: resourceMonitorSubscriptionDiagnosticSettingDelete,

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := ParseMonitorDiagnosticId(id)
			return err
		}),

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(60 * time.Minute),
		},

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.MonitorDiagnosticSettingName,
			},

			"target_subscription_id": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
			},

			"eventhub_name": {
				Type:         pluginsdk.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: eventhubValidate.ValidateEventHubName(),
			},

			"eventhub_authorization_rule_id": {
				Type:         pluginsdk.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: authRuleParse.ValidateAuthorizationRuleID,
			},

			"log_analytics_workspace_id": {
				Type:         pluginsdk.TypeString,
				Optional:     true,
				ValidateFunc: workspaces.ValidateWorkspaceID,
			},

			"storage_account_id": {
				Type:         pluginsdk.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: storageValidate.StorageAccountID,
			},

			"log": {
				Type:     pluginsdk.TypeSet,
				Optional: true,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"category": {
							Type:     pluginsdk.TypeString,
							Optional: true,
						},

						"category_group": {
							Type:     pluginsdk.TypeString,
							Optional: true,
						},

						"enabled": {
							Type:     pluginsdk.TypeBool,
							Optional: true,
							Default:  true,
						},
					},
				},
				Set: resourceMonitorSubscriptionDiagnosticLogSettingHash,
			},
		},
	}
}

func resourceMonitorSubscriptionDiagnosticSettingCreateUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Monitor.SubscriptionDiagnosticSettingsClient
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()
	log.Printf("[INFO] preparing arguments for Azure ARM Diagnostic Settings.")

	name := d.Get("name").(string)
	subscriptionID := d.Get("target_subscription_id").(string)
	subscriptionDiagnosticSettingId := subscriptiondiagnosticsettings.NewDiagnosticSettingID(subscriptionID, name)

	if d.IsNewResource() {
		existing, err := client.Get(ctx, subscriptionDiagnosticSettingId)
		if err != nil {
			if !response.WasNotFound(existing.HttpResponse) {
				return fmt.Errorf("checking for presence of existing Monitor Subscription Diagnostic Setting %q for Subscription %q: %s", subscriptionDiagnosticSettingId.Name, subscriptionDiagnosticSettingId.SubscriptionId, err)
			}
		}

		if existing.Model != nil && existing.Model.Id != nil && *existing.Model.Id != "" {
			return tf.ImportAsExistsError("azurerm_monitor_subscription_diagnostic_setting", *existing.Model.Id)
		}
	}

	logsRaw := d.Get("log").(*pluginsdk.Set).List()
	logs := expandSubscriptionMonitorDiagnosticsSettingsLogs(logsRaw)
	// if no blocks are specified  the API "creates" but 404's on Read
	if len(logs) == 0 {
		return fmt.Errorf("At least one `log` block must be specified")
	}

	// also if there's none enabled
	valid := false
	for _, v := range logs {
		if v.Enabled {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("At least one `log` must be enabled")
	}

	parameters := subscriptiondiagnosticsettings.SubscriptionDiagnosticSettingsResource{
		Properties: &subscriptiondiagnosticsettings.SubscriptionDiagnosticSettings{
			Logs: &logs,
		},
	}

	valid = false
	eventHubAuthorizationRuleId := d.Get("eventhub_authorization_rule_id").(string)
	eventHubName := d.Get("eventhub_name").(string)
	if eventHubAuthorizationRuleId != "" {
		parameters.Properties.EventHubAuthorizationRuleId = utils.String(eventHubAuthorizationRuleId)
		parameters.Properties.EventHubName = utils.String(eventHubName)
		valid = true
	}

	workspaceId := d.Get("log_analytics_workspace_id").(string)
	if workspaceId != "" {
		parameters.Properties.WorkspaceId = utils.String(workspaceId)
		valid = true
	}

	storageAccountId := d.Get("storage_account_id").(string)
	if storageAccountId != "" {
		parameters.Properties.StorageAccountId = utils.String(storageAccountId)
		valid = true
	}

	if !valid {
		return fmt.Errorf("Either a `eventhub_authorization_rule_id`, `log_analytics_workspace_id` or `storage_account_id` must be set")
	}

	if _, err := client.CreateOrUpdate(ctx, subscriptionDiagnosticSettingId, parameters); err != nil {
		return fmt.Errorf("creating Monitor Subscription Diagnostics Setting %q for Subscription %q: %+v", name, subscriptionID, err)
	}

	read, err := client.Get(ctx, subscriptionDiagnosticSettingId)
	if err != nil {
		return err
	}
	if read.Model == nil && read.Model.Id == nil {
		return fmt.Errorf("Cannot read ID for Monitor Diagnostics %q for Subscription ID %q", subscriptionDiagnosticSettingId.Name, subscriptionDiagnosticSettingId.SubscriptionId)
	}

	d.SetId(fmt.Sprintf("%s|%s", subscriptionID, name))

	return resourceMonitorSubscriptionDiagnosticSettingRead(d, meta)
}

func resourceMonitorSubscriptionDiagnosticSettingRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Monitor.SubscriptionDiagnosticSettingsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := ParseMonitorSubscriptionDiagnosticId(d.Id())
	if err != nil {
		return err
	}

	subscriptionId := id.SubscriptionId
	resp, err := client.Get(ctx, *id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			log.Printf("[WARN] Monitor Subscription Diagnostics Setting %q was not found for Subscription %q - removing from state!", id.Name, subscriptionId)
			d.SetId("")
			return nil
		}

		return fmt.Errorf("retrieving Monitor Subscription Diagnostics Setting %q for Resource %q: %+v", id.Name, subscriptionId, err)
	}

	d.Set("name", id.Name)
	d.Set("target_subscription_id", id.SubscriptionId)

	if model := resp.Model; model != nil {
		if props := model.Properties; props != nil {
			d.Set("eventhub_name", props.EventHubName)
			eventhubAuthorizationRuleId := ""
			if props.EventHubAuthorizationRuleId != nil && *props.EventHubAuthorizationRuleId != "" {
				authRuleId := utils.NormalizeNilableString(props.EventHubAuthorizationRuleId)
				parsedId, err := authRuleParse.ParseAuthorizationRuleIDInsensitively(authRuleId)
				if err != nil {
					return err
				}
				eventhubAuthorizationRuleId = parsedId.ID()
			}
			d.Set("eventhub_authorization_rule_id", eventhubAuthorizationRuleId)

			workspaceId := ""
			if props.WorkspaceId != nil && *props.WorkspaceId != "" {
				parsedId, err := workspaces.ParseWorkspaceID(*props.WorkspaceId)
				if err != nil {
					return err
				}

				workspaceId = parsedId.ID()
			}
			d.Set("log_analytics_workspace_id", workspaceId)

			storageAccountId := ""
			if props.StorageAccountId != nil && *props.StorageAccountId != "" {
				parsedId, err := storageParse.StorageAccountID(*props.StorageAccountId)
				if err != nil {
					return err
				}

				storageAccountId = parsedId.ID()
				d.Set("storage_account_id", storageAccountId)
			}

			if err := d.Set("log", flattenMonitorSubscriptionDiagnosticLogs(resp.Model.Properties.Logs)); err != nil {
				return fmt.Errorf("setting `log`: %+v", err)
			}
		}
	}

	return nil
}

func resourceMonitorSubscriptionDiagnosticSettingDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Monitor.SubscriptionDiagnosticSettingsClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := ParseMonitorSubscriptionDiagnosticId(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Delete(ctx, *id)
	if err != nil {
		if !response.WasNotFound(resp.HttpResponse) {
			return fmt.Errorf("deleting Monitor Subscription Diagnostics Setting %q for Subscription %q: %+v", id.Name, id.SubscriptionId, err)
		}
	}

	// API appears to be eventually consistent (identified during tainting this resource)
	log.Printf("[DEBUG] Waiting for Monitor Subscription Diagnostic Setting %q for Subscription %q to disappear", id.Name, id.SubscriptionId)
	stateConf := &pluginsdk.StateChangeConf{
		Pending:                   []string{"Exists"},
		Target:                    []string{"NotFound"},
		Refresh:                   monitorSubscriptionDiagnosticSettingDeletedRefreshFunc(ctx, client, *id),
		MinTimeout:                15 * time.Second,
		ContinuousTargetOccurence: 5,
		Timeout:                   d.Timeout(pluginsdk.TimeoutDelete),
	}

	if _, err = stateConf.WaitForStateContext(ctx); err != nil {
		return fmt.Errorf("waiting for Monitor Diagnostic Setting %q for Subscription %q to become available: %s", id.Name, id.SubscriptionId, err)
	}

	return nil
}

func monitorSubscriptionDiagnosticSettingDeletedRefreshFunc(ctx context.Context, client *subscriptiondiagnosticsettings.SubscriptionDiagnosticSettingsClient, targetSettingId subscriptiondiagnosticsettings.DiagnosticSettingId) pluginsdk.StateRefreshFunc {
	return func() (interface{}, string, error) {
		res, err := client.Get(ctx, targetSettingId)
		if err != nil {
			if response.WasNotFound(res.HttpResponse) {
				return "NotFound", "NotFound", nil
			}
			return nil, "", fmt.Errorf("issuing read request in monitorSubscriptionDiagnosticSettingDeletedRefreshFunc: %s", err)
		}

		return res, "Exists", nil
	}
}

func expandSubscriptionMonitorDiagnosticsSettingsLogs(input []interface{}) []subscriptiondiagnosticsettings.SubscriptionLogSettings {
	results := make([]subscriptiondiagnosticsettings.SubscriptionLogSettings, 0)

	for _, raw := range input {
		v := raw.(map[string]interface{})

		category := v["category"].(string)
		categoryGroup := v["category_group"].(string)
		enabled := v["enabled"].(bool)

		output := subscriptiondiagnosticsettings.SubscriptionLogSettings{
			Enabled: enabled,
		}
		if category != "" {
			output.Category = utils.String(category)
		} else {
			output.CategoryGroup = utils.String(categoryGroup)
		}

		results = append(results, output)
	}

	return results
}

func flattenMonitorSubscriptionDiagnosticLogs(input *[]subscriptiondiagnosticsettings.SubscriptionLogSettings) []interface{} {
	results := make([]interface{}, 0)
	if input == nil {
		return results
	}

	for _, v := range *input {
		output := make(map[string]interface{})

		if v.Category != nil {
			output["category"] = *v.Category
		}

		if v.CategoryGroup != nil {
			output["category_group"] = *v.CategoryGroup
		}

		output["enabled"] = v.Enabled

		results = append(results, output)
	}

	return results
}

func ParseMonitorSubscriptionDiagnosticId(monitorId string) (*subscriptiondiagnosticsettings.DiagnosticSettingId, error) {
	v := strings.Split(monitorId, "|")
	if len(v) != 2 {
		return nil, fmt.Errorf("Expected the Monitor Subscription Diagnostics ID to be in the format `{subscriptionId}|{name}` but got %d segments", len(v))
	}
	return &subscriptiondiagnosticsettings.DiagnosticSettingId{
		SubscriptionId: v[0],
		Name:           v[1],
	}, nil
}

func resourceMonitorSubscriptionDiagnosticLogSettingHash(input interface{}) int {
	var buf bytes.Buffer
	if rawData, ok := input.(map[string]interface{}); ok {
		if category, ok := rawData["category"]; ok {
			buf.WriteString(fmt.Sprintf("%s-", category.(string)))
		}
		if categoryGroup, ok := rawData["category_group"]; ok {
			buf.WriteString(fmt.Sprintf("%s-", categoryGroup.(string)))
		}
		if enabled, ok := rawData["enabled"]; ok {
			buf.WriteString(fmt.Sprintf("%t-", enabled.(bool)))
		}
	}
	return pluginsdk.HashString(buf.String())
}
