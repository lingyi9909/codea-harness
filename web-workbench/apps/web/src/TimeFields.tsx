/**
 * 通用时间输入组件
 *
 * 职责：统一管理日期与时间的录入和格式转换。
 * Java 研发阅读提示：注意业务时区、日期边界以及浏览器本地时间的差异。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
/** Shared native date and second-precision clock controls used by investigations and component tests. */
type Props={label:string;value:string;onChange:(value:string)=>void;invalid?:boolean;descriptionId?:string;required?:boolean};
export function CalendarDate({label,value,onChange,invalid=false,descriptionId,required=true}:Props){
  return <label>{label}<input type="date" aria-label={label} value={value} required={required}
    aria-invalid={invalid} aria-describedby={descriptionId}
    onChange={e=>onChange(e.target.value)}/></label>;
}
export function SecondClock({label,value,onChange,invalid=false,descriptionId,required=true}:Props){
  return <label>{label}<input type="text" aria-label={label} inputMode="numeric" value={value}
    maxLength={8} placeholder="HH:mm:ss" autoComplete="off" required={required}
    aria-invalid={invalid} aria-describedby={descriptionId}
    onChange={e=>onChange(e.target.value)}/></label>;
}
export function localDayBounds(day:string){
  if(!/^\d{4}-\d{2}-\d{2}$/.test(day))return null;
  const [year,month,date]=day.split('-').map(Number);
  const start=new Date(year,month-1,date,0,0,0);
  if(start.getFullYear()!==year||start.getMonth()+1!==month||start.getDate()!==date)return null;
  const end=new Date(year,month-1,date+1,0,0,0);
  return {from:start.getTime(),to:end.getTime(),durationHours:(end.getTime()-start.getTime())/3600000};
}
export function parseSecondDateTime(date:string,clock:string){
  if(!/^\d{2}:\d{2}:\d{2}$/.test(clock)||!localDayBounds(date))return null;
  const [hours,minutes,seconds]=clock.split(':').map(Number);
  if(hours>23||minutes>59||seconds>59)return null;
  const parsed=new Date(date+'T'+clock);
  if(!Number.isFinite(parsed.getTime())||parsed.getHours()!==hours||parsed.getMinutes()!==minutes||parsed.getSeconds()!==seconds)return null;
  return parsed.getTime();
}
