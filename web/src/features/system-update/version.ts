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

export function normalizeVersion(version: string): string {
  return version.trim().replace(/^v/i, '').split('+', 1)[0]
}

export function compareVersions(left: string, right: string): number {
  const [leftCore, leftPrerelease] = normalizeVersion(left).split('-', 2)
  const [rightCore, rightPrerelease] = normalizeVersion(right).split('-', 2)
  const leftCoreParts = leftCore.split('.')
  const rightCoreParts = rightCore.split('.')
  const coreLength = Math.max(leftCoreParts.length, rightCoreParts.length)

  for (let index = 0; index < coreLength; index += 1) {
    const leftPart = leftCoreParts[index] ?? '0'
    const rightPart = rightCoreParts[index] ?? '0'
    const comparison = compareVersionIdentifier(leftPart, rightPart)
    if (comparison !== 0) return comparison
  }

  if (!leftPrerelease && !rightPrerelease) return 0
  if (!leftPrerelease) return 1
  if (!rightPrerelease) return -1

  const leftPrereleaseParts = leftPrerelease.split('.')
  const rightPrereleaseParts = rightPrerelease.split('.')
  const prereleaseLength = Math.max(
    leftPrereleaseParts.length,
    rightPrereleaseParts.length
  )
  for (let index = 0; index < prereleaseLength; index += 1) {
    const leftPart = leftPrereleaseParts[index]
    const rightPart = rightPrereleaseParts[index]
    if (leftPart === undefined) return -1
    if (rightPart === undefined) return 1
    const comparison = compareVersionIdentifier(leftPart, rightPart, true)
    if (comparison !== 0) return comparison
  }
  return 0
}

function compareVersionIdentifier(
  left: string,
  right: string,
  prerelease = false
): number {
  const leftNumeric = /^\d+$/.test(left)
  const rightNumeric = /^\d+$/.test(right)
  if (leftNumeric && rightNumeric) {
    const normalizedLeft = left.replace(/^0+(?=\d)/, '')
    const normalizedRight = right.replace(/^0+(?=\d)/, '')
    if (normalizedLeft.length !== normalizedRight.length) {
      return normalizedLeft.length > normalizedRight.length ? 1 : -1
    }
    if (normalizedLeft === normalizedRight) return 0
    return normalizedLeft > normalizedRight ? 1 : -1
  }
  if (prerelease && leftNumeric !== rightNumeric) return leftNumeric ? -1 : 1
  const comparison = left.localeCompare(right)
  if (comparison === 0) return 0
  return comparison > 0 ? 1 : -1
}
