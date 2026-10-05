import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
HTMLElement.prototype.scrollIntoView = vi.fn();
import Ask from '../pages/Ask';
import KnowledgePanel from '../components/KnowledgePanel';
import { WhoContext } from '../session';
import { mockApi } from '../test/mock';

const document = {id:'stock',title:'Stock SOP',source:'Operations handbook',text:'Reorder at ten units.',visibility:'shared',roles:[],version:1,sha256:'abc123',updated_at:'2026-10-05T00:00:00Z',updated_by:'admin'};
const admin = {identity:{subject:'admin',role:'admin' as const,roles:['admin' as const],method:'password'},auth_required:true};

it('allows an admin to create a document with explicit visibility and version', async () => {
 const calls=mockApi({'GET /api/v1/knowledge/documents':{documents:[]},'PUT /api/v1/knowledge/documents/stock':document});
 render(<WhoContext.Provider value={admin}><KnowledgePanel /></WhoContext.Provider>);
 fireEvent.click(screen.getByRole('button',{name:'Add document'}));
 fireEvent.change(screen.getByLabelText('Document ID'),{target:{value:'stock'}});
 fireEvent.change(screen.getByLabelText('Document title'),{target:{value:'Stock SOP'}});
 fireEvent.change(screen.getByLabelText('Document text'),{target:{value:'Reorder at ten units.'}});
 fireEvent.click(screen.getByRole('button',{name:'Save document'}));
 expect(await screen.findByText('Document saved.')).toBeTruthy();
 const request=calls.find(c=>c.method==='PUT');
 expect(request?.body).toMatchObject({expected_version:0,document:{id:'stock',visibility:'provider'}});
});

it('keeps tenant viewers read-only and displays source provenance', async () => {
 mockApi({'GET /api/v1/knowledge/documents':{documents:[document]},'GET /api/v1/knowledge/documents/stock':document});
 render(<WhoContext.Provider value={{identity:{subject:'alice',role:'viewer',tenant:'alpha',method:'password'},auth_required:true}}><KnowledgePanel /></WhoContext.Provider>);
 fireEvent.click(await screen.findByRole('button',{name:'Stock SOP'}));
 expect(await screen.findByText('SHA-256 abc123')).toBeTruthy();
 expect(screen.queryByRole('button',{name:'Add document'})).toBeNull();
 expect(screen.queryByRole('button',{name:'Delete document'})).toBeNull();
});

it('asks explicitly from documents and displays immutable citations', async () => {
 const calls=mockApi({
  'GET /api/v1/ai/status':{mode:'heuristic',mutations:'never'},
  'GET /api/v1/ontology/actions':{actions:[]},
  'GET /api/v1/knowledge/documents':{documents:[]},
  'POST /api/v1/ai/ask':{text:'Relevant evidence.',intent:'documents',mode:'heuristic',grounding:['document:stock@1'],document_citations:[{document:'stock',title:'Stock SOP',source:'Operations handbook',version:1,sha256:'abc123',start_line:1,end_line:1,excerpt:'Reorder at ten units.',score:1}]},
 });
 render(<Ask />);
 fireEvent.change(screen.getByLabelText('Answer source'),{target:{value:'documents'}});
 fireEvent.change(screen.getByLabelText('Your question'),{target:{value:'When to reorder?'}});
 fireEvent.click(screen.getByRole('button',{name:'Ask'}));
 expect(await screen.findByText('Document citations (1)')).toBeTruthy();
 expect(screen.getByText('Reorder at ten units.')).toBeTruthy();
 await waitFor(()=>expect(calls.find(c=>c.method==='POST')?.body).toEqual({question:'When to reorder?',scope:'documents'}));
});
