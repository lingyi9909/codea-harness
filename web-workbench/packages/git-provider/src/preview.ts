import type { DTO } from '../../contracts/src/domain.ts';
import { git, GitFailure, getRefs } from './repository.ts';
import type { Ref } from './repository.ts';
const OID=/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;
export interface PreviewProof{
  scope:DTO<'Scope'>; changedFiles:number; applicableFiles:number;
  dirtyCounts:DTO<'DirtyCounts'>; exclusions:DTO<'Exclusion'>[]; notice:string; ref:Ref;
}
export async function previewCommittedChanges(root:string,repoId:string,secret:string,baseRefId:string):Promise<PreviewProof>{
  const refs=await getRefs(root,repoId,secret);
  const base=refs.items.find(ref=>ref.id===baseRefId);
  if(!base)throw new GitFailure('BASE_REF_MISSING');
  let mergeBaseOID:string;
  try{mergeBaseOID=await git(root,['merge-base',base.oid,refs.headOID]);}
  catch{throw new GitFailure('NO_COMMON_ANCESTOR');}
  if(!OID.test(mergeBaseOID))throw new GitFailure('INVALID_MERGE_BASE');
  const [diff,dirty]=await Promise.all([
    git(root,['diff','--no-ext-diff','--no-textconv','--name-only','-z',
      mergeBaseOID,refs.headOID,'--']),
    git(root,['status','--porcelain=v1','-z','--untracked-files=all']),
  ]);
  const changed=[...new Set(diff.split('\0').filter(Boolean))];
  if(!changed.length)throw new GitFailure('NO_CHANGES');
  const applicable=changed.filter(f=>!/(^|\/)\.(?:opencode|teamai|git)(\/|$)/.test(f)
    && (/\.(?:java|xml|sql|yml|yaml|properties)$/i.test(f)||/(^|\/)pom\.xml$/i.test(f)));
  if(!applicable.length)throw new GitFailure('NO_APPLICABLE_FILES');
  let staged=0,unstaged=0,untracked=0;
  for(const entry of dirty.split('\0').filter(Boolean)){
    const flag=entry.slice(0,2);
    if(flag==='??')untracked++;
    else{
      if(flag[0]!==' '&&flag[0]!=='?')staged++;
      if(flag[1]!==' '&&flag[1]!=='?')unstaged++;
    }
  }
  const scope:DTO<'Scope'>={headOID:refs.headOID,baseOID:base.oid,
    mergeBaseOID,baseRef:base.fullName,sourceBranch:refs.branch,objectFormat:refs.objectFormat};
  return {scope,changedFiles:changed.length,applicableFiles:applicable.length,
    dirtyCounts:{staged,unstaged,untracked},
    exclusions:changed.filter(f=>!applicable.includes(f)).map(path=>({
      path,reason:'非业务代码文件或隐藏配置目录，已排除',
    })),
    notice:'审查只包含已提交变更；未提交文件不会纳入，远端跟踪 ref 仅代表本地缓存。',
    ref:base};
}
