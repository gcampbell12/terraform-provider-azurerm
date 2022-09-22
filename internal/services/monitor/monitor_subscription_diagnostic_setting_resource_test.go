package monitor_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/monitor"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

type MonitorSubscriptionDiagnosticSettingResource struct{}

func TestAccMonitorSubscriptionDiagnosticSetting_eventhub(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_monitor_subscription_diagnostic_setting", "test")
	r := MonitorSubscriptionDiagnosticSettingResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.eventhub(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("eventhub_name").Exists(),
				check.That(data.ResourceName).Key("eventhub_authorization_rule_id").Exists(),
				check.That(data.ResourceName).Key("log.#").HasValue("2"),
			),
		},
		data.ImportStep(),
	})
}

func TestAccMonitorSubscriptionDiagnosticSetting_CategoryGroup(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_monitor_subscription_diagnostic_setting", "test")
	r := MonitorSubscriptionDiagnosticSettingResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.categoryGroup(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("eventhub_name").Exists(),
				check.That(data.ResourceName).Key("eventhub_authorization_rule_id").Exists(),
				check.That(data.ResourceName).Key("log.#").HasValue("1"),
			),
		},
		data.ImportStep(),
	})
}

func TestAccMonitorSubscriptionDiagnosticSetting_requiresImport(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_monitor_subscription_diagnostic_setting", "test")
	r := MonitorSubscriptionDiagnosticSettingResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.eventhub(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		{
			Config:      r.requiresImport(data),
			ExpectError: acceptance.RequiresImportError("azurerm_monitor_subscription_diagnostic_setting"),
		},
	})
}

func TestAccMonitorSubscriptionDiagnosticSetting_logAnalyticsWorkspace(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_monitor_subscription_diagnostic_setting", "test")
	r := MonitorSubscriptionDiagnosticSettingResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.logAnalyticsWorkspace(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("log_analytics_workspace_id").Exists(),
				check.That(data.ResourceName).Key("log.#").HasValue("2"),
			),
		},
		data.ImportStep(),
	})
}

func TestAccMonitorSubscriptionDiagnosticSetting_storageAccount(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_monitor_subscription_diagnostic_setting", "test")
	r := MonitorSubscriptionDiagnosticSettingResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.storageAccount(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("storage_account_id").Exists(),
				check.That(data.ResourceName).Key("log.#").HasValue("2"),
			),
		},
		data.ImportStep(),
	})
}

func TestAccMonitorSubscriptionDiagnosticSetting_activityLog(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_monitor_subscription_diagnostic_setting", "test")
	r := MonitorSubscriptionDiagnosticSettingResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.activityLog(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
	})
}

func (t MonitorSubscriptionDiagnosticSettingResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := monitor.ParseMonitorSubscriptionDiagnosticId(state.ID)
	if err != nil {
		return nil, err
	}

	resp, err := clients.Monitor.SubscriptionDiagnosticSettingsClient.Get(ctx, *id)
	if err != nil {
		return nil, fmt.Errorf("reading subscription diagnostic setting (%s): %+v", id, err)
	}

	return utils.Bool(resp.Model != nil && resp.Model.Id != nil), nil
}

func (MonitorSubscriptionDiagnosticSettingResource) eventhub(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

data "azurerm_subscription" "current" {
}

resource "azurerm_resource_group" "test" {
  name     = "acctestRG-%[1]d"
  location = "%[2]s"
}

resource "azurerm_eventhub_namespace" "test" {
  name                = "acctest-EHN-%[1]d"
  location            = azurerm_resource_group.test.location
  resource_group_name = azurerm_resource_group.test.name
  sku                 = "Basic"
}

resource "azurerm_eventhub" "test" {
  name                = "acctest-EH-%[1]d"
  namespace_name      = azurerm_eventhub_namespace.test.name
  resource_group_name = azurerm_resource_group.test.name
  partition_count     = 2
  message_retention   = 1
}

resource "azurerm_eventhub_namespace_authorization_rule" "test" {
  name                = "example"
  namespace_name      = azurerm_eventhub_namespace.test.name
  resource_group_name = azurerm_resource_group.test.name
  listen              = true
  send                = true
  manage              = true
}

resource "azurerm_monitor_subscription_diagnostic_setting" "test" {
  name                           = "acctest-DS-%[1]d"
  target_subscription_id         = data.azurerm_subscription.current.subscription_id
  eventhub_authorization_rule_id = azurerm_eventhub_namespace_authorization_rule.test.id
  eventhub_name                  = azurerm_eventhub.test.name

  log {
    category = "Security"
    enabled  = true
  }

  log {
    category = "Administrative"
    enabled  = false
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomIntOfLength(17))
}

func (MonitorSubscriptionDiagnosticSettingResource) categoryGroup(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

data "azurerm_subscription" "current" {
}

resource "azurerm_resource_group" "test" {
  name     = "acctestRG-%[1]d"
  location = "%[2]s"
}

resource "azurerm_eventhub_namespace" "test" {
  name                = "acctest-EHN-%[1]d"
  location            = azurerm_resource_group.test.location
  resource_group_name = azurerm_resource_group.test.name
  sku                 = "Basic"
}

resource "azurerm_eventhub" "test" {
  name                = "acctest-EH-%[1]d"
  namespace_name      = azurerm_eventhub_namespace.test.name
  resource_group_name = azurerm_resource_group.test.name
  partition_count     = 2
  message_retention   = 1
}

resource "azurerm_eventhub_namespace_authorization_rule" "test" {
  name                = "example"
  namespace_name      = azurerm_eventhub_namespace.test.name
  resource_group_name = azurerm_resource_group.test.name
  listen              = true
  send                = true
  manage              = true
}

resource "azurerm_monitor_subscription_diagnostic_setting" "test" {
  name                           = "acctest-DS-%[1]d"
  target_subscription_id         = data.azurerm_subscription.current.subscription_id
  eventhub_authorization_rule_id = azurerm_eventhub_namespace_authorization_rule.test.id
  eventhub_name                  = azurerm_eventhub.test.name

  log {
    category_group = "allLogs"
    enabled        = true
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomIntOfLength(17))
}
func (r MonitorSubscriptionDiagnosticSettingResource) requiresImport(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

data "azurerm_subscription" "current" {
}

resource "azurerm_monitor_subscription_diagnostic_setting" "import" {
  name                           = azurerm_monitor_subscription_diagnostic_setting.test.name
  target_subscription_id         = data.azurerm_subscription.current.subscription_id
  eventhub_authorization_rule_id = azurerm_monitor_subscription_diagnostic_setting.test.eventhub_authorization_rule_id
  eventhub_name                  = azurerm_monitor_subscription_diagnostic_setting.test.eventhub_name

  log {
    category = "Security"
    enabled  = true
  }
}
`, r.eventhub(data))
}

func (MonitorSubscriptionDiagnosticSettingResource) logAnalyticsWorkspace(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

data "azurerm_subscription" "current" {
}

resource "azurerm_resource_group" "test" {
  name     = "acctestRG-%[1]d"
  location = "%[2]s"
}

resource "azurerm_log_analytics_workspace" "test" {
  name                = "acctest-LAW-%[1]d"
  location            = azurerm_resource_group.test.location
  resource_group_name = azurerm_resource_group.test.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
}

resource "azurerm_monitor_subscription_diagnostic_setting" "test" {
  name                       = "acctest-DS-%[1]d"
  target_subscription_id     = data.azurerm_subscription.current.subscription_id
  log_analytics_workspace_id = azurerm_log_analytics_workspace.test.id

  log {
    category = "Administrative"
    enabled  = false
  }

  log {
    category = "Security"
    enabled  = true
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomIntOfLength(17))
}

func (MonitorSubscriptionDiagnosticSettingResource) storageAccount(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

data "azurerm_subscription" "current" {
}

resource "azurerm_resource_group" "test" {
  name     = "acctestRG-%[1]d"
  location = "%[2]s"
}

resource "azurerm_storage_account" "test" {
  name                     = "acctest%[3]d"
  resource_group_name      = azurerm_resource_group.test.name
  location                 = azurerm_resource_group.test.location
  account_replication_type = "LRS"
  account_tier             = "Standard"
}

resource "azurerm_monitor_subscription_diagnostic_setting" "test" {
  name                   = "acctest-DS-%[1]d"
  target_subscription_id = data.azurerm_subscription.current.subscription_id
  storage_account_id     = azurerm_storage_account.test.id

  log {
    category = "Administrative"
    enabled  = true
  }

  log {
    category = "Security"
    enabled  = false
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomIntOfLength(17))
}

func (MonitorSubscriptionDiagnosticSettingResource) activityLog(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

data "azurerm_subscription" "current" {
}

resource "azurerm_resource_group" "test" {
  name     = "acctestRG-%[1]d"
  location = "%[2]s"
}

resource "azurerm_storage_account" "test" {
  name                     = "acctest%[3]d"
  resource_group_name      = azurerm_resource_group.test.name
  location                 = azurerm_resource_group.test.location
  account_replication_type = "LRS"
  account_tier             = "Standard"
}


resource "azurerm_monitor_subscription_diagnostic_setting" "test" {
  name                   = "acctest-DS-%[1]d"
  target_subscription_id = data.azurerm_subscription.current.subscription_id
  storage_account_id     = azurerm_storage_account.test.id

  log {
    category = "Administrative"
  }

  log {
    category = "Security"
  }

  log {
    category = "ServiceHealth"
  }

  log {
    category = "Alert"
  }

  log {
    category = "Recommendation"
  }

  log {
    category = "Policy"
  }

  log {
    category = "Autoscale"
  }

  log {
    category = "ResourceHealth"
    enabled  = false
  }
}
`, data.RandomInteger, data.Locations.Primary, data.RandomIntOfLength(17))
}
