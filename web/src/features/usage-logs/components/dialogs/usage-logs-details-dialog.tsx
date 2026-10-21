/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useLogsViewScope, useUsageLogsContext } from '../usage-logs-provider'
import { DetailsDialog } from './details-dialog'

/**
 * Page-level host for the log details dialog. The open entry lives in the logs
 * provider instead of a table cell so the auto-refresh (which re-creates column
 * renderers and rows) cannot unmount the dialog and close it.
 */
export function UsageLogsDetailsDialog() {
  const { detailsLog, detailsDialogOpen, setDetailsDialogOpen } =
    useUsageLogsContext()
  const { isAdminView } = useLogsViewScope()

  if (!detailsLog) return null

  return (
    <DetailsDialog
      log={detailsLog}
      isAdmin={isAdminView}
      open={detailsDialogOpen}
      onOpenChange={setDetailsDialogOpen}
    />
  )
}
