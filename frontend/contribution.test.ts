import { beforeEach, describe, expect, it, vi } from 'vitest'
const host=vi.hoisted(()=>({register:vi.fn(),get:vi.fn(),mounted:vi.fn()}))
vi.mock('@solovey/component-ui',()=>({registerComponent:host.register,HttpUtils:{get:host.get},i18n:{global:{t:(key:string)=>key}}}))
vi.mock('@solovey/component-ui/observation',()=>({useUiMode:()=>({mode:{value:'nexus'}})}))
vi.mock('notivue',()=>({push:{success:vi.fn(),error:vi.fn()}}))
vi.mock('vue',async original=>({...await original<typeof import('vue')>(),onMounted:host.mounted}))
import { register } from './index'
import { useDiagnosticsPage } from './composables/useDiagnosticsPage'
import en from './locales/en'
import fa from './locales/fa'
import ru from './locales/ru'
import viLocale from './locales/vi'
import zhcn from './locales/zhcn'
import zhtw from './locales/zhtw'
const keys=(v:Record<string,unknown>,prefix=''):string[]=>Object.entries(v).flatMap(([k,x])=>typeof x==='object'&&x!==null?keys(x as Record<string,unknown>,prefix+k+'.'):[prefix+k]).sort()
beforeEach(()=>vi.clearAllMocks())
describe('local authorized UI contribution',()=>{
 it('preserves routes, permissions, navigation and six locale loaders',()=>{register();const c=host.register.mock.calls[0]![0];expect(c.id).toBe('observability-extra');expect(c.routes.map((r:Record<string,unknown>)=>r.path)).toEqual(['/audit','/diagnostics']);expect(c.routes.map((r:{access:unknown})=>r.access)).toEqual([{hostScope:'local',surface:'operations',permission:'audit.read'},{hostScope:'local',surface:'operations',permission:'diagnostics.read'}]);expect(c.nav.map((n:{order:number})=>n.order)).toEqual([120,121]);expect(Object.keys(c.locales).sort()).toEqual(['en','fa','ru','vi','zhHans','zhHant'])})
 it.each([en,fa,ru,viLocale,zhcn,zhtw])('retains the existing locale contribution and English fallback',locale=>{const actual=keys(locale);expect(actual).toEqual(expect.arrayContaining(['audit.title','diagnostics.title']));expect(actual.every(key=>keys(en).includes(key))).toBe(true)})
 it('reads core reports and clamps Log Inspector requests without owning the data',async()=>{const page=useDiagnosticsPage();host.get.mockResolvedValueOnce({success:true,obj:{generatedAt:1700,health:{status:'healthy',counts:{ok:3}},checks:[]}}).mockResolvedValueOnce({success:true,obj:[]});await page.refreshAll();expect(host.get).toHaveBeenCalledWith('api/diagnostics/report');expect(page.loading.value).toBe(false);expect(page.counts.value.ok).toBe(3);page.logFilters.value.count=10000;host.get.mockResolvedValueOnce({success:true,obj:[]});await page.loadLogs();expect(host.get).toHaveBeenLastCalledWith('api/logs/entries',expect.objectContaining({count:500}));expect(page.logsLoading.value).toBe(false)})
 it('retains report and log errors and clears loading after rejection',async()=>{const page=useDiagnosticsPage();host.get.mockResolvedValue({success:false,msg:'denied'});await page.refreshAll();expect(page.error.value).toBe('denied');host.get.mockRejectedValueOnce(Error('offline'));await expect(page.loadLogs()).rejects.toThrow('offline');expect(page.logsLoading.value).toBe(false)})
})
