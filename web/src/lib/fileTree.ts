export interface FileLeaf {
  path: string
  covered: number
  relevant: number
}

export interface TreeNode {
  name: string
  path: string
  isDir: boolean
  covered: number
  relevant: number
  children: TreeNode[]
}

function newDir(name: string, path: string): TreeNode {
  return { name, path, isDir: true, covered: 0, relevant: 0, children: [] }
}

// buildTree builds a directory tree from flat file paths, aggregating
// covered/relevant counts up to every ancestor folder. Children are sorted
// folders-first, then alphabetically.
export function buildTree(files: FileLeaf[]): TreeNode {
  const root = newDir('', '')
  for (const f of files) {
    const parts = f.path.split('/')
    let node = root
    node.covered += f.covered
    node.relevant += f.relevant
    let prefix = ''
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      prefix = prefix ? `${prefix}/${part}` : part
      const last = i === parts.length - 1
      if (last) {
        node.children.push({ name: part, path: f.path, isDir: false, covered: f.covered, relevant: f.relevant, children: [] })
      } else {
        let child = node.children.find((c) => c.isDir && c.name === part)
        if (!child) {
          child = newDir(part, prefix)
          node.children.push(child)
        }
        child.covered += f.covered
        child.relevant += f.relevant
        node = child
      }
    }
  }
  sortTree(root)
  return root
}

function sortTree(node: TreeNode) {
  node.children.sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
    return a.name.localeCompare(b.name)
  })
  for (const c of node.children) if (c.isDir) sortTree(c)
}
