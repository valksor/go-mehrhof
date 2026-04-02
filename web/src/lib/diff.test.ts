import { describe, it, expect } from 'vitest'
import { parseDiffForFile } from './diff'

describe('parseDiffForFile', () => {
  it('returns null for empty diff', () => {
    expect(parseDiffForFile('', 'file.ts')).toBeNull()
  })

  it('returns null when file is not in the diff', () => {
    const diff = `diff --git a/other.ts b/other.ts
index abc..def 100644
--- a/other.ts
+++ b/other.ts
@@ -1,3 +1,3 @@
 line1
-old
+new
 line3`
    expect(parseDiffForFile(diff, 'file.ts')).toBeNull()
  })

  it('parses a simple addition', () => {
    const diff = `diff --git a/file.ts b/file.ts
index abc..def 100644
--- a/file.ts
+++ b/file.ts
@@ -1,2 +1,3 @@
 line1
+added
 line2`
    const result = parseDiffForFile(diff, 'file.ts')
    expect(result).toEqual({
      oldValue: 'line1\nline2',
      newValue: 'line1\nadded\nline2',
    })
  })

  it('parses a simple removal', () => {
    const diff = `diff --git a/file.ts b/file.ts
index abc..def 100644
--- a/file.ts
+++ b/file.ts
@@ -1,3 +1,2 @@
 line1
-removed
 line2`
    const result = parseDiffForFile(diff, 'file.ts')
    expect(result).toEqual({
      oldValue: 'line1\nremoved\nline2',
      newValue: 'line1\nline2',
    })
  })

  it('parses a modification', () => {
    const diff = `diff --git a/file.ts b/file.ts
index abc..def 100644
--- a/file.ts
+++ b/file.ts
@@ -1,3 +1,3 @@
 line1
-old
+new
 line3`
    const result = parseDiffForFile(diff, 'file.ts')
    expect(result).toEqual({
      oldValue: 'line1\nold\nline3',
      newValue: 'line1\nnew\nline3',
    })
  })

  it('extracts the correct file from a multi-file diff', () => {
    const diff = `diff --git a/first.ts b/first.ts
index abc..def 100644
--- a/first.ts
+++ b/first.ts
@@ -1,2 +1,2 @@
-old1
+new1
diff --git a/second.ts b/second.ts
index abc..def 100644
--- a/second.ts
+++ b/second.ts
@@ -1,2 +1,2 @@
-old2
+new2`
    const result = parseDiffForFile(diff, 'second.ts')
    expect(result).toEqual({
      oldValue: 'old2',
      newValue: 'new2',
    })
  })

  it('handles diff with renamed file (matches right path)', () => {
    const diff = `diff --git a/old-name.ts b/new-name.ts
index abc..def 100644
--- a/old-name.ts
+++ b/new-name.ts
@@ -1,2 +1,2 @@
-old
+new`
    const result = parseDiffForFile(diff, 'new-name.ts')
    expect(result).toEqual({
      oldValue: 'old',
      newValue: 'new',
    })
  })

  it('handles diff with renamed file (matches left path)', () => {
    const diff = `diff --git a/old-name.ts b/new-name.ts
index abc..def 100644
--- a/old-name.ts
+++ b/new-name.ts
@@ -1,2 +1,2 @@
-old
+new`
    const result = parseDiffForFile(diff, 'old-name.ts')
    expect(result).toEqual({
      oldValue: 'old',
      newValue: 'new',
    })
  })

  it('returns null for a file with only metadata lines (no hunks)', () => {
    const diff = `diff --git a/file.ts b/file.ts
index abc..def 100644
--- a/file.ts
+++ b/file.ts`
    expect(parseDiffForFile(diff, 'file.ts')).toBeNull()
  })

  it('skips index, ---, +++, and @@ lines', () => {
    const diff = `diff --git a/file.ts b/file.ts
index 1234567..abcdef0 100644
--- a/file.ts
+++ b/file.ts
@@ -10,3 +10,3 @@ function foo() {
 context
-old
+new`
    const result = parseDiffForFile(diff, 'file.ts')
    expect(result).toEqual({
      oldValue: 'context\nold',
      newValue: 'context\nnew',
    })
  })
})
