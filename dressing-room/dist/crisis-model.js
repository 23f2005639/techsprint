/* Deterministic educational model. Real concurrency requires a server transaction. */
(function(root){
  'use strict';
  const catalog={hoodie:'Oversized hoodie',tee:'Oversized tee'};
  const key=(item,size)=>`${item}:${size}`;
  function create(){return {interests:[{id:'i-a',customer:'Aarav',item:'hoodie',size:'L',quantity:1},{id:'i-m',customer:'Meera',item:'hoodie',size:'L',quantity:1},{id:'i-r',customer:'Riya',item:'hoodie',size:'M',quantity:1},{id:'i-k',customer:'Kabir',item:'hoodie',size:'XL',quantity:1}],stock:{'hoodie:L':1,'hoodie:M':2,'hoodie:XL':2,'tee:M':2,'tee:L':2,'tee:XL':2},holds:[],requests:{},events:[],converted:[],queues:{}};}
  function interest(s,entry){
    if(!catalog[entry.item]||!['M','L','XL'].includes(entry.size)||!Number.isInteger(entry.quantity)||entry.quantity<1||entry.quantity>10||!entry.id||!entry.customer)return {ok:false};
    if(s.interests.some(x=>x.id===entry.id))return {ok:true,replay:true};
    s.interests.push({...entry});return {ok:true};
  }
  function demand(s){const totals={};for(const x of s.interests){const k=key(x.item,x.size);totals[k]=(totals[k]||0)+x.quantity;}return totals;}
  function reserve(s,customer,item,size,quantity,requestId,now=0){
    if(!requestId||!catalog[item]||!['M','L','XL'].includes(size)||!Number.isInteger(quantity)||quantity<1)return {ok:false,reason:'invalid'};
    const signature=JSON.stringify([customer,item,size,quantity]);
    if(Object.hasOwn(s.requests,requestId))return s.requests[requestId].signature===signature?{...s.requests[requestId].result,replay:true}:{ok:false,reason:'request-conflict'};
    const k=key(item,size);
    if(s.queues[k]?.length&&s.queues[k][0]!==customer)return {ok:false,reason:'waitlist-priority'};
    if((s.stock[k]||0)<quantity)return {ok:false,reason:'unavailable'};
    s.stock[k]-=quantity;
    const hold={id:requestId,customer,item,size,quantity,status:'held',expiresAt:now+86400};
    s.holds.push(hold);if(s.queues[k]?.[0]===customer)s.queues[k].shift();const result={ok:true,id:requestId};s.requests[requestId]={signature,result};
    if(s.interests.some(x=>x.customer===customer)&&!s.converted.includes(customer))s.converted.push(customer);
    s.events.push({type:'hold-confirmed',id:requestId});return result;
  }
  function expire(s,id,now){const h=s.holds.find(x=>x.id===id);if(!h||h.status!=='held'||now<h.expiresAt)return false;h.status='expired';s.stock[key(h.item,h.size)]+=h.quantity;s.events.push({type:'released',id});const next=s.queues[key(h.item,h.size)]?.[0];if(next)s.events.push({type:'priority-notification',id,customer:next});return true;}
  function collect(s,id,now){const h=s.holds.find(x=>x.id===id);if(!h||h.status!=='held'||now>=h.expiresAt)return false;h.status='sold';s.events.push({type:'collected',id});return true;}
  function metrics(s){const customers=new Set(s.interests.map(x=>x.customer));return {customers:customers.size,converted:s.converted.length,conversion:customers.size?Math.round(s.converted.length/customers.size*100):0,uncollected:s.holds.filter(x=>x.status==='held').length};}
  function reminder(s,id,now){const h=s.holds.find(x=>x.id===id);if(!h||h.status!=='held'||now>=h.expiresAt)return null;const hours=Math.ceil((h.expiresAt-now)/3600);return `Hi ${h.customer}, your ${catalog[h.item].toLowerCase()} (size ${h.size}, qty ${h.quantity}) is held at The Dressing Room. Please collect within ${hours} ${hours===1?'hour':'hours'}. Your hold will expire after that. Show your hold ID ${h.id} at the store. Directions: https://www.google.com/maps/search/?api=1&query=The%20Dressing%20Room%20Indore`;}
  function enqueue(s,customer,item,size){const k=key(item,size);s.queues[k] ||= [];if(!s.queues[k].includes(customer))s.queues[k].push(customer);return s.queues[k].indexOf(customer)+1;}
  function remind(s,id,now){const h=s.holds.find(x=>x.id===id);if(!h||h.status!=='held'||now<h.expiresAt-7200||now>=h.expiresAt)return null;let e=s.events.find(x=>x.type==='pickup-reminder'&&x.id===id);if(e)return {...e,replay:true};e={type:'pickup-reminder',id,channel:'SMS / Push',text:reminder(s,id,now)};s.events.push(e);return e;}
  root.CrisisModel={catalog,create,interest,demand,reserve,expire,collect,metrics,reminder,enqueue,remind};
  if(typeof module!=='undefined'&&module.exports)module.exports=root.CrisisModel;
})(typeof window!=='undefined'?window:globalThis);
