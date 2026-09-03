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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { compareVersions } from './version'

describe('system update version comparison', () => {
  test('orders stable and prerelease versions using semver precedence', () => {
    assert.equal(compareVersions('v1.2.3', '1.2.3-rc.1'), 1)
    assert.equal(compareVersions('1.2.3-rc.2', '1.2.3-rc.10'), -1)
    assert.equal(compareVersions('1.2.3-1', '1.2.3-alpha'), -1)
  })

  test('ignores build metadata and compares large numeric identifiers exactly', () => {
    assert.equal(compareVersions('1.2.3+build.2', 'v1.2.3+build.1'), 0)
    assert.equal(
      compareVersions('1.2.9007199254740993', '1.2.9007199254740992'),
      1
    )
  })
})
