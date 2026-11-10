import fs from 'node:fs'
import path from 'node:path'
import assert from 'node:assert/strict'
import {createHash} from 'node:crypto'
import {execFileSync} from 'node:child_process'
import {fileURLToPath} from 'node:url'

const root=fileURLToPath(new URL('../',import.meta.url))
assert.equal(process.env.GOWORK,'off','qualification requires GOWORK=off')
assert.ok((process.env.GOFLAGS??'').split(/\s+/).every(flag=>!flag||flag==='-mod=readonly'),'source qualification rejects Go flag substitution')
const mod=JSON.parse(execFileSync('go',['mod','edit','-json'],{cwd:root,encoding:'utf8'}))
assert.equal(mod.Module.Path,'github.com/MalenkiySolovey/solovey-component-observability-extra')
assert.ok(!(mod.Replace??[]).length,'source module replacements are forbidden')
assert.deepEqual(Object.fromEntries((mod.Require??[]).map(item=>[item.Path,item.Version])),{'github.com/MalenkiySolovey/solovey-host':'v0.2.0','github.com/robfig/cron/v3':'v3.0.1'},'unreviewed source module requirements')
const source=JSON.parse(fs.readFileSync(path.join(root,'SOURCE_PROVENANCE.json')))
assert.equal(source.component,'observability-extra')
assert.equal(source.originalCommit,'0bbdde2d4381453a966dd2f015ef0ac1c4773ccf')
assert.equal(createHash('sha256').update(fs.readFileSync(path.join(root,'LICENSE'))).digest('hex'),source.licenseSHA256,'original license changed')
execFileSync('git',['merge-base','--is-ancestor',source.filteredHistoryCommit,'HEAD'],{cwd:root,stdio:'pipe'})
for(const record of source.files) {
 assert.ok(record.destination&&!record.destination.includes('..')&&!path.isAbsolute(record.destination))
 assert.equal(createHash('sha256').update(fs.readFileSync(path.join(root,record.destination))).digest('hex'),record.destinationSHA256,`mapped source drift: ${record.destination}`)
}
const output=execFileSync('go',['list','-deps','-test','-json','./...'],{cwd:root,encoding:'utf8',maxBuffer:16*1024*1024})
const packages=(output.match(/^{[\s\S]*?^}/gm)??[]).map(JSON.parse)
assert.ok(packages.length>0)
const allowed=new Set(['github.com/MalenkiySolovey/solovey-component-observability-extra','github.com/MalenkiySolovey/solovey-host','github.com/robfig/cron/v3'])
for(const pkg of packages)if(pkg.Module){assert.ok(allowed.has(pkg.Module.Path),`unapproved source dependency ${pkg.ImportPath}`);assert.ok(!pkg.Module.Replace,'source module substitution')}
const walk=directory=>fs.readdirSync(directory,{withFileTypes:true}).flatMap(entry=>entry.name==='node_modules'||entry.name.startsWith('.')?[]:entry.isDirectory()?walk(path.join(directory,entry.name)):[path.join(directory,entry.name)])
for(const filename of walk(root).filter(name=>/\.(go|ts|vue)$/.test(name))) {
 const text=fs.readFileSync(filename,'utf8')
 assert.ok(!text.includes('github.com/MalenkiySolovey/solovey-ui/'),`Panel dependency ${filename}`)
 assert.ok(!/from\s+['"]@\//.test(text),`private UI import ${filename}`)
 assert.ok(!/\b(?:M[026][A-C]?|R7|Wave|Phase|Goal)[-_]\w+/.test(text),`development-stage identifier ${filename}`)
}
console.log(JSON.stringify({status:'PASS',packages:packages.length,allowedModules:[...allowed],mappedSources:source.files.length,privatePanelImports:0,filteredHistoryPreserved:true}))
