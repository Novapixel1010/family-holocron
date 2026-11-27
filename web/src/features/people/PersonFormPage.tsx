import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { toPersonInput, useCreatePerson, usePerson, useUpdatePerson } from "@/api/hooks";
import { useAuth } from "@/app/auth";
import { PageContainer } from "@/app/Layout";
import { Card, PageHeader } from "@/components/ui/Page";
import { QueryBoundary } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { personName } from "@/lib/format";
import { emptyPerson, PersonForm } from "./PersonForm";

export function NewPersonPage() {
  const { canEdit } = useAuth();
  const create = useCreatePerson();
  const navigate = useNavigate();
  const toast = useToast();
  if (!canEdit) return <Navigate to="/people" replace />;
  return (
    <PageContainer>
      <PageHeader title="Add a person" subtitle={<Link to="/people" className="hover:underline">← Back to people</Link>} />
      <Card>
        <PersonForm
          initial={emptyPerson}
          submitLabel="Add person"
          onCancel={() => navigate(-1)}
          onSubmit={async (input) => {
            const p = await create.mutateAsync(input);
            toast(`${personName(p)} added`);
            navigate(`/people/${p.id}`);
          }}
        />
      </Card>
    </PageContainer>
  );
}

export function EditPersonPage() {
  const { canEdit } = useAuth();
  const id = Number(useParams().id);
  const person = usePerson(id);
  const update = useUpdatePerson();
  const navigate = useNavigate();
  const toast = useToast();
  if (!canEdit) return <Navigate to={`/people/${id}`} replace />;
  return (
    <PageContainer>
      <QueryBoundary query={person} errorTitle="Couldn't load this person">
        {(p) => (
          <>
            <PageHeader title={`Edit ${personName(p)}`} subtitle={<Link to={`/people/${id}`} className="hover:underline">← Back to profile</Link>} />
            <Card>
              <PersonForm
                initial={toPersonInput(p)}
                submitLabel="Save changes"
                onCancel={() => navigate(`/people/${id}`)}
                onSubmit={async (input) => {
                  await update.mutateAsync({ id, input });
                  toast("Changes saved");
                  navigate(`/people/${id}`);
                }}
              />
            </Card>
          </>
        )}
      </QueryBoundary>
    </PageContainer>
  );
}
