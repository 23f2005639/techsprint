import pathlib,urllib.request,concurrent.futures
root=pathlib.Path(__file__).resolve().parents[1]/'web/public/assets'
photos={'cargo':'1473966968600-fa801b869a1a','tee':'1521572163474-6864f9cf17ab','varsity':'1551028719-00167b16eac5','bag':'1553062407-98eeb64c6a62','denim':'1543076447-215ad9ba6923','beanie':'1576871337632-b9aef4c17ab9'}
def fetch(item):
 name,photo=item;p=root/(name+'.jpg')
 if not p.exists():
  with urllib.request.urlopen('https://images.unsplash.com/photo-'+photo+'?w=700&q=80&fit=crop',timeout=60) as r:p.write_bytes(r.read())
 print(name,p.stat().st_size)
with concurrent.futures.ThreadPoolExecutor(max_workers=6) as ex:list(ex.map(fetch,photos.items()))
