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
export type SponsorSite = {
  enabled: boolean
  name: string
  url: string
}

export function parseSponsorSites(raw: unknown): SponsorSite[] {
  const sites = Array.isArray(raw) ? raw : []
  return Array.from({ length: 3 }, (_, index) => {
    const site = sites[index] as Partial<SponsorSite> | null | undefined
    return {
      enabled: site?.enabled === true,
      name: typeof site?.name === 'string' ? site.name.trim() : '',
      url: typeof site?.url === 'string' ? site.url.trim() : '',
    }
  })
}
